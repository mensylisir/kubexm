package pki

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kubexms/kubexms/pkg/connector"
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"
	"github.com/kubexms/kubexms/pkg/step/spec"
	// Assuming DefaultEtcdPKIPathKey is defined in determine_etcd_pki_path.go or a constants file
)

// SharedData key for the list of fetched etcd certificate file basenames.
const (
	DefaultEtcdFetchedFilesListKey = "etcdFetchedCertFiles"
)

// FetchExistingEtcdCertsStepSpec defines parameters for fetching existing etcd certificates.
type FetchExistingEtcdCertsStepSpec struct {
	TargetPKIPathSharedDataKey string `json:"targetPKIPathSharedDataKey,omitempty"` // Local path to store fetched certs
	RemoteCertDir              string `json:"remoteCertDir,omitempty"`              // Remote directory where etcd certs are stored
	OutputFetchedFilesListKey  string `json:"outputFetchedFilesListKey,omitempty"`  // SharedData key for the list of fetched file basenames
}

// GetName returns the name of the step.
func (s *FetchExistingEtcdCertsStepSpec) GetName() string {
	return "Fetch Existing Etcd Certificates"
}

// PopulateDefaults sets default values for the spec.
func (s *FetchExistingEtcdCertsStepSpec) PopulateDefaults() {
	if s.TargetPKIPathSharedDataKey == "" {
		s.TargetPKIPathSharedDataKey = DefaultEtcdPKIPathKey // From determine_etcd_pki_path.go
	}
	if s.RemoteCertDir == "" {
		s.RemoteCertDir = "/etc/ssl/etcd/ssl" // Common default for etcd certs
	}
	if s.OutputFetchedFilesListKey == "" {
		s.OutputFetchedFilesListKey = DefaultEtcdFetchedFilesListKey
	}
}

// FetchExistingEtcdCertsStepExecutor implements the logic.
type FetchExistingEtcdCertsStepExecutor struct{}

// Check determines if etcd certificates seem to have been already fetched.
func (e *FetchExistingEtcdCertsStepExecutor) Check(s spec.StepSpec, ctx *runtime.Context) (isDone bool, err error) {
	stepSpec, ok := s.(*FetchExistingEtcdCertsStepSpec)
	if !ok {
		return false, fmt.Errorf("unexpected spec type %T for %s", s, stepSpec.GetName())
	}
	stepSpec.PopulateDefaults()
	logger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step", stepSpec.GetName())

	targetPKIPathVal, pkiPathOk := ctx.SharedData.Load(stepSpec.TargetPKIPathSharedDataKey)
	if !pkiPathOk {
		logger.Debugf("Target PKI path not found in SharedData key '%s'. Cannot check.", stepSpec.TargetPKIPathSharedDataKey)
		return false, nil
	}
	targetPKIPath, ok := targetPKIPathVal.(string)
	if !ok || targetPKIPath == "" {
		logger.Warnf("Invalid or empty target PKI path in SharedData key '%s'.", stepSpec.TargetPKIPathSharedDataKey)
		return false, nil
	}

	fetchedFilesRaw, listExists := ctx.SharedData.Load(stepSpec.OutputFetchedFilesListKey)
	if !listExists {
		logger.Debugf("List of fetched files (key: '%s') not found in SharedData. Assuming not done.", stepSpec.OutputFetchedFilesListKey)
		return false, nil
	}

	fetchedFiles, ok := fetchedFilesRaw.([]string)
	if !ok {
		logger.Errorf("Invalid type for fetched files list in SharedData key '%s', expected []string, got %T.", stepSpec.OutputFetchedFilesListKey, fetchedFilesRaw)
		return false, fmt.Errorf("invalid type for fetched files list in SharedData key '%s'", stepSpec.OutputFetchedFilesListKey)
	}

	if len(fetchedFiles) == 0 {
		// This could mean either no certs were found on the remote, or the step hasn't run successfully.
		// If the expectation is that certs *should* exist and be fetched, then an empty list means "not done".
		// If it's possible no certs exist remotely and that's OK, this logic might need adjustment
		// or Execute should store a specific marker if it ran and found nothing.
		logger.Debugf("Fetched files list (key: '%s') is empty. Assuming not done or no files were fetched.", stepSpec.OutputFetchedFilesListKey)
		return false, nil
	}

	for _, fileName := range fetchedFiles {
		localPath := filepath.Join(targetPKIPath, fileName)
		if _, statErr := os.Stat(localPath); os.IsNotExist(statErr) { // Local filesystem check
			logger.Infof("Previously fetched file %s not found at local path %s. Re-fetch needed.", fileName, localPath)
			return false, nil
		} else if statErr != nil {
			return false, fmt.Errorf("failed to stat fetched file %s: %w", localPath, statErr)
		}
	}

	logger.Infof("All previously fetched etcd certificate files found in local PKI path %s.", targetPKIPath)
	return true, nil
}

// Execute fetches existing etcd certificates from the remote host.
func (e *FetchExistingEtcdCertsStepExecutor) Execute(s spec.StepSpec, ctx *runtime.Context) *step.Result {
	stepSpec, ok := s.(*FetchExistingEtcdCertsStepSpec)
	if !ok {
		return step.NewResultForSpec(s, fmt.Errorf("unexpected spec type %T", s))
	}
	stepSpec.PopulateDefaults()
	logger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step", stepSpec.GetName())
	res := step.NewResultForSpec(s, nil) // Result associated with the remote host context

	targetPKIPathVal, pkiPathOk := ctx.SharedData.Load(stepSpec.TargetPKIPathSharedDataKey)
	if !pkiPathOk {
		res.Error = fmt.Errorf("target PKI path for storing certs not found in SharedData key '%s'", stepSpec.TargetPKIPathSharedDataKey)
		res.SetFailed(); return res
	}
	targetPKIPath, typeOk := targetPKIPathVal.(string)
	if !typeOk || targetPKIPath == "" {
		res.Error = fmt.Errorf("invalid or empty target PKI path in SharedData key '%s'", stepSpec.TargetPKIPathSharedDataKey)
		res.SetFailed(); return res
	}

	// Ensure local targetPKIPath directory exists (local filesystem operation)
	if err := os.MkdirAll(targetPKIPath, 0700); err != nil {
		res.Error = fmt.Errorf("failed to create local target PKI directory %s: %w", targetPKIPath, err)
		res.SetFailed(); return res
	}
	logger.Infof("Ensured local PKI directory exists: %s", targetPKIPath)

	// List files in the remote directory
	// `ls -p` appends '/' to directory names. Grep filters for common cert extensions. `|| true` prevents failure if no files match.
	lsCmd := fmt.Sprintf("ls -p %s 2>/dev/null | grep -E '\\.pem$|\\.crt$|\\.key$' || true", stepSpec.RemoteCertDir)
	logger.Infof("Listing remote certificate files in %s on host %s using command: %s", stepSpec.RemoteCertDir, ctx.Host.Name, lsCmd)

	// Sudo might be needed to list contents of /etc/ssl/etcd/ssl
	certListStr, stdErrStr, errLs := ctx.Host.Runner.RunWithOptions(ctx.GoContext, lsCmd, &connector.ExecOptions{Sudo: true, AllowFailure: true})
	if errLs != nil {
		// Even with `|| true`, RunWithOptions might return an error if the command itself is not found, or for other runner issues.
		logger.Warnf("Command '%s' to list remote certs failed or had issues (stderr: %s): %v. Proceeding cautiously.", lsCmd, stdErrStr, errLs)
		// Depending on error, might still have partial output in certListStr if `|| true` worked as intended in shell.
	}
	if stdErrStr != "" && !strings.Contains(stdErrStr,"No such file or directory") { // Ignore "No such file" as `|| true` handles it.
		logger.Warnf("Stderr from remote list command: %s", stdErrStr)
	}


	rawFiles := strings.Split(strings.TrimSpace(certListStr), "\n")
	fetchedFileBaseNames := []string{}
	var errorsEncountered []string

	if len(rawFiles) == 1 && rawFiles[0] == "" { // No files found by ls/grep
		rawFiles = []string{} // Ensure it's an empty slice, not a slice with one empty string
	}

	logger.Infof("Found %d potential file(s) to fetch from remote directory %s.", len(rawFiles), stepSpec.RemoteCertDir)

	for _, fileNameFromLs := range rawFiles {
		trimmedName := strings.TrimSpace(fileNameFromLs)
		if trimmedName == "" || strings.HasSuffix(trimmedName, "/") { // Skip empty lines or directories
			continue
		}

		remoteSrc := filepath.Join(stepSpec.RemoteCertDir, trimmedName)
		localDst := filepath.Join(targetPKIPath, trimmedName)

		logger.Infof("Fetching remote file %s to local path %s", remoteSrc, localDst)
		// Assuming Runner.Fetch(ctx, localDestination, remoteSource) error
		// Sudo for Fetch is usually not needed for remote source if readable, and localDest is already created.
		fetchErr := ctx.Host.Runner.Fetch(ctx.GoContext, localDst, remoteSrc)
		if fetchErr != nil {
			errMsg := fmt.Sprintf("failed to fetch %s to %s: %v", remoteSrc, localDst, fetchErr)
			logger.Warn(errMsg)
			errorsEncountered = append(errorsEncountered, errMsg)
			continue // Try to fetch other files
		}
		logger.Successf("Successfully fetched %s to %s", remoteSrc, localDst)
		fetchedFileBaseNames = append(fetchedFileBaseNames, trimmedName)
	}

	ctx.SharedData.Store(stepSpec.OutputFetchedFilesListKey, fetchedFileBaseNames)
	logger.Infof("List of fetched file basenames stored in SharedData key '%s': %v", stepSpec.OutputFetchedFilesListKey, fetchedFileBaseNames)

	if len(errorsEncountered) > 0 {
		errMsg := fmt.Sprintf("encountered %d error(s) while fetching etcd certificates: %s", len(errorsEncountered), strings.Join(errorsEncountered, "; "))
		// If all attempts to fetch files failed, and we expected files, it's a failure.
		if len(fetchedFileBaseNames) == 0 && len(rawFiles) > 0 {
			res.Error = fmt.Errorf(errMsg)
			res.SetFailed(); return res
		}
		// Otherwise, it's a partial success, treat as success with warnings for now.
		// A more sophisticated approach might involve comparing expected certs vs fetched.
		logger.Warn(errMsg) // Log as warning if some files were fetched.
		// res.Message = errMsg // Append to success message or set a specific status
	}

	if len(fetchedFileBaseNames) == 0 && len(rawFiles) > 0 && len(errorsEncountered) == len(rawFiles) {
		res.Error = fmt.Errorf("all attempts to fetch %d listed remote files failed. Errors: %s", len(rawFiles), strings.Join(errorsEncountered, "; "))
		res.SetFailed(); return res
	}

	if len(fetchedFileBaseNames) == 0 && len(rawFiles) == 0 {
	    logger.Info("No etcd certificate files found in remote directory to fetch.")
	}


	// Perform post-execution check
	done, checkErr := e.Check(s, ctx)
	if checkErr != nil {
		res.Error = fmt.Errorf("post-execution check failed: %w", checkErr)
		res.SetFailed(); return res
	}
	if !done {
		// This could happen if fetching succeeded but Check logic has an issue or files were removed post-fetch.
		// Or if no files were fetched and Check expects some.
		res.Error = fmt.Errorf("post-execution check indicates etcd certificate fetching was not fully successful or verification failed")
		res.SetFailed(); return res
	}

	res.SetSucceeded(fmt.Sprintf("Fetched %d etcd certificate files to %s.", len(fetchedFileBaseNames), targetPKIPath))
	if len(errorsEncountered) > 0 {
		res.Message += fmt.Sprintf(" Encountered %d errors during fetch.", len(errorsEncountered))
	}
	return res
}

func init() {
	step.Register(&FetchExistingEtcdCertsStepSpec{}, &FetchExistingEtcdCertsStepExecutor{})
}
