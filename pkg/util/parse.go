package util

import (
	"fmt"
	"regexp"
)

func ParseCaCertHashFromOutput(output string) (string, error) {
	re := regexp.MustCompile(`--discovery-token-ca-cert-hash\s+sha256:([a-f0-9]{64})`)
	matches := re.FindStringSubmatch(output)
	if len(matches) != 2 {
		return "", fmt.Errorf("ca cert hash (sha256:...) not found in kubeadm output")
	}
	return matches[1], nil
}

func ParseTokenFromOutput(output string) (string, error) {
	re := regexp.MustCompile(`--token\s+([a-z0-9]{6}\.[a-z0-9]{16})`)
	matches := re.FindStringSubmatch(output)
	if len(matches) != 2 {
		return "", fmt.Errorf("bootstrap token not found in kubeadm output")
	}
	return matches[1], nil
}

func ParseCertificateKeyFromOutput(output string) (string, error) {
	re := regexp.MustCompile(`--certificate-key\s+([a-f0-9]{64})`)
	matches := re.FindStringSubmatch(output)
	if len(matches) != 2 {
		return "", fmt.Errorf("certificate key not found in kubeadm output, did you use --upload-certs?")
	}
	return matches[1], nil
}