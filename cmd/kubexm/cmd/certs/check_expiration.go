package certs

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common" // For default directory names
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/fatih/color" // For colorized output
)

// CheckExpirationOptions holds options for the check-expiration command
type CheckExpirationOptions struct {
	ClusterName   string
	WarnWithinDays int
}

var checkExpirationOptions = &CheckExpirationOptions{}

// pkiPathForCluster returns the conventional PKI path for a given cluster name.
func pkiPathForCluster(clusterName string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	// $HOME/.kubexm/clusters/<cluster_name>/pki
	return filepath.Join(homeDir, common.KubexmRootDirName, "clusters", clusterName, "pki"), nil
}

// Define a list of well-known certificate relative paths within the PKI directory.
// This list might need to be adjusted based on how kubexm actually structures its PKI.
var knownCertificatePaths = []string{
	"ca.crt",
	"apiserver.crt",
	"apiserver.key", // Key files won't be parsed for expiration but good to list
	"apiserver-kubelet-client.crt",
	"apiserver-kubelet-client.key",
	"front-proxy-ca.crt",
	"front-proxy-client.crt",
	"front-proxy-client.key",
	"etcd/ca.crt", // Etcd CA might be same as main CA or separate
	"etcd/server.crt",
	"etcd/server.key",
	"etcd/peer.crt",
	"etcd/peer.key",
	"etcd/healthcheck-client.crt",
	"etcd/healthcheck-client.key",
	// Add other certs like kubelet client certs if managed this way, service account CAs etc.
}

type CertDetail struct {
	FilePath      string
	FileName      string // Relative path from PKI root
	Subject       string
	Issuer        string
	NotBefore     time.Time
	NotAfter      time.Time
	ExpiresIn     time.Duration
	IsCA          bool
	IsProblematic bool // Expired or expiring soon
}

func init() {
	CertsCmd.AddCommand(checkExpirationCmd)
	checkExpirationCmd.Flags().StringVarP(&checkExpirationOptions.ClusterName, "cluster", "c", "", "Name of the cluster to check certificates for (required)")
	checkExpirationCmd.Flags().IntVar(&checkExpirationOptions.WarnWithinDays, "warn-within", 30, "Warn if a certificate is expiring within this many days.")

	if err := checkExpirationCmd.MarkFlagRequired("cluster"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to mark 'cluster' flag as required for 'certs check-expiration': %v\n", err)
	}
}

var checkExpirationCmd = &cobra.Command{
	Use:   "check-expiration",
	Short: "Check expiration dates of cluster certificates",
	Long: `Checks the expiration dates of important PKI certificates for a specified cluster,
assuming they are stored in the conventional local path: $HOME/.kubexm/clusters/[CLUSTER_NAME]/pki/`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if checkExpirationOptions.ClusterName == "" {
			return fmt.Errorf("cluster name must be specified via --cluster or -c flag")
		}

		pkiDir, err := pkiPathForCluster(checkExpirationOptions.ClusterName)
		if err != nil {
			return err
		}

		if _, err := os.Stat(pkiDir); os.IsNotExist(err) {
			return fmt.Errorf("PKI directory not found for cluster '%s' at %s. Ensure the cluster was created and PKI exists", checkExpirationOptions.ClusterName, pkiDir)
		}

		fmt.Printf("Checking certificate expirations for cluster '%s' in %s\n", checkExpirationOptions.ClusterName, pkiDir)
		fmt.Printf("Warning threshold: Certificates expiring within %d days.\n\n", checkExpirationOptions.WarnWithinDays)

		var certDetails []CertDetail
		foundAnyCerts := false

		warnThresholdDuration := time.Duration(checkExpirationOptions.WarnWithinDays) * 24 * time.Hour

		// Walk the PKI directory to find all .crt files, not just the known ones.
		// This is more robust if the PKI structure varies slightly or has extra certs.
		err = filepath.WalkDir(pkiDir, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				fmt.Fprintf(os.Stderr, "Error accessing path %s: %v\n", path, walkErr)
				return walkErr // Or return nil to continue walking other parts
			}
			if d.IsDir() || !strings.HasSuffix(d.Name(), ".crt") {
				return nil // Skip directories and non-certificate files
			}

			foundAnyCerts = true
			fileContent, err := os.ReadFile(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to read certificate file %s: %v\n", path, err)
				// Add a problematic entry? For now, just log and skip.
				return nil // Continue walking
			}

			block, _ := pem.Decode(fileContent)
			if block == nil {
				// fmt.Fprintf(os.Stderr, "Failed to decode PEM block from %s\n", path)
				// This can happen for key files if we were also looking for .key, or empty/corrupt .crt
				// For .crt, it's usually an issue. We can add it as a problematic entry.
				certDetails = append(certDetails, CertDetail{
					FilePath: path,
					FileName: strings.TrimPrefix(path, pkiDir+string(filepath.Separator)),
					Subject: "N/A (Failed to decode PEM)",
					IsProblematic: true,
				})
				return nil // Continue
			}

			certs, err := x509.ParseCertificates(block.Bytes)
			if err != nil {
				// fmt.Fprintf(os.Stderr, "Failed to parse certificate from %s: %v\n", path, err)
				certDetails = append(certDetails, CertDetail{
					FilePath: path,
					FileName: strings.TrimPrefix(path, pkiDir+string(filepath.Separator)),
					Subject: "N/A (Failed to parse x509 cert)",
					IsProblematic: true,
				})
				return nil // Continue
			}

			for _, cert := range certs {
				expiresIn := time.Until(cert.NotAfter)
				isProblematic := expiresIn <= 0 // Expired
				if !isProblematic && expiresIn < warnThresholdDuration { // Expiring soon
					isProblematic = true
				}

				detail := CertDetail{
					FilePath:      path,
					FileName:      strings.TrimPrefix(path, pkiDir+string(filepath.Separator)),
					Subject:       cert.Subject.CommonName, // Using CommonName for brevity, could use full Subject.String()
					Issuer:        cert.Issuer.CommonName,  // Similarly for Issuer
					NotBefore:     cert.NotBefore,
					NotAfter:      cert.NotAfter,
					ExpiresIn:     expiresIn,
					IsCA:          cert.IsCA,
					IsProblematic: isProblematic,
				}
				certDetails = append(certDetails, detail)
			}
			return nil
		})

		if err != nil {
			// This error is from filepath.WalkDir itself, not from parsing individual files if we returned nil there.
			return fmt.Errorf("error walking PKI directory %s: %w", pkiDir, err)
		}


		if !foundAnyCerts {
			fmt.Println("No certificate (.crt) files found in the PKI directory.")
			return nil
		}

		// Sort by expiration date (earliest first)
		sort.Slice(certDetails, func(i, j int) bool {
			if certDetails[i].IsProblematic && !certDetails[j].IsProblematic {
				return true // Problematic ones first
			}
			if !certDetails[i].IsProblematic && certDetails[j].IsProblematic {
				return false
			}
			// If both problematic or both not, sort by actual expiration
			return certDetails[i].NotAfter.Before(certDetails[j].NotAfter)
		})

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"CERTIFICATE", "SUBJECT", "ISSUER", "EXPIRES IN", "EXPIRATION DATE", "IS CA"})
		table.SetBorder(true)
		table.SetColumnSeparator("│")
		table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
		table.SetAlignment(tablewriter.ALIGN_LEFT)

		red := color.New(color.FgRed).SprintFunc()
		yellow := color.New(color.FgYellow).SprintFunc()

		for _, detail := range certDetails {
			expiresInStr := "N/A"
			notAfterStr := "N/A"
			if detail.Subject != "N/A (Failed to decode PEM)" && detail.Subject != "N/A (Failed to parse x509 cert)" { // Check if parsing was successful
				if detail.ExpiresIn <= 0 {
					expiresInStr = red(fmt.Sprintf("EXPIRED (%s ago)", formatDuration(detail.ExpiresIn.Abs())))
				} else {
					expiresInStr = formatDuration(detail.ExpiresIn)
					if detail.ExpiresIn < warnThresholdDuration {
						expiresInStr = yellow(expiresInStr)
					}
				}
				notAfterStr = detail.NotAfter.Format("Jan 02, 2006 15:04 MST")
				if detail.IsProblematic && detail.ExpiresIn > 0 { // Expiring soon but not yet expired
					notAfterStr = yellow(notAfterStr)
				} else if detail.ExpiresIn <=0 { // Expired
					notAfterStr = red(notAfterStr)
				}
			} else { // Handle cases where parsing failed
				expiresInStr = red("ERROR")
				notAfterStr = red("ERROR")
			}


			isCAStr := "no"
			if detail.IsCA {
				isCAStr = "yes"
			}

			row := []string{
				detail.FileName,
				detail.Subject,
				detail.Issuer,
				expiresInStr,
				notAfterStr,
				isCAStr,
			}
			table.Append(row)
		}
		table.Render()

		return nil
	},
}

// formatDuration formats duration into a human-readable string like "30d", "2h", "5m".
// This is a simplified version.
func formatDuration(d time.Duration) string {
	if d < 0 {
		return "0s (already passed)"
	}
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	// seconds := int(d.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd", days)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh", hours)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%ds", int(d.Seconds()))
}
