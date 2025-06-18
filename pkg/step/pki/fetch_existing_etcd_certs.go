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
func (e *FetchExistingEtcdCertsStepExecutor) Check(ctx runtime.Context) (isDone bool, err error) {
	currentFullSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return false, fmt.Errorf("StepSpec not found in context for FetchExistingEtcdCertsStep Check")
	}
	spec, ok := currentFullSpec.(*FetchExistingEtcdCertsStepSpec)
	if !ok {
		return false, fmt.Errorf("unexpected StepSpec type for FetchExistingEtcdCertsStep Check: %T", currentFullSpec)
	}
	spec.PopulateDefaults()
	logger := ctx.Logger.SugaredLogger().With("host", ctx.Host.Name, "step", spec.GetName())

	targetPKIPathVal, pkiPathOk := ctx.Task().Get(spec.TargetPKIPathSharedDataKey)
	if !pkiPathOk {
		logger.Debugf("Target PKI path not found in Task Cache key '%s'. Cannot check.", spec.TargetPKIPathSharedDataKey)
		return false, nil
	}
	targetPKIPath, ok := targetPKIPathVal.(string)
	if !ok || targetPKIPath == "" {
		logger.Warnf("Invalid or empty target PKI path in Task Cache key '%s'.", spec.TargetPKIPathSharedDataKey)
		return false, nil
	}

	fetchedFilesRaw, listExists := ctx.Task().Get(spec.OutputFetchedFilesListKey)
	if !listExists {
		logger.Debugf("List of fetched files (key: '%s') not found in Task Cache. Assuming not done.", spec.OutputFetchedFilesListKey)
		return false, nil
	}

	fetchedFiles, ok := fetchedFilesRaw.([]string)
	if !ok {
		logger.Errorf("Invalid type for fetched files list in Task Cache key '%s', expected []string, got %T.", spec.OutputFetchedFilesListKey, fetchedFilesRaw)
		return false, fmt.Errorf("invalid type for fetched files list in Task Cache key '%s'", spec.OutputFetchedFilesListKey)
	}

	if len(fetchedFiles) == 0 {
		logger.Debugf("Fetched files list (key: '%s') is empty. Assuming not done or no files were fetched.", spec.OutputFetchedFilesListKey)
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
func (e *FetchExistingEtcdCertsStepExecutor) Execute(ctx runtime.Context) *step.Result {
	startTime := time.Now()
	currentFullSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("StepSpec not found in context for FetchExistingEtcdCertsStep Execute"))
	}
	spec, ok := currentFullSpec.(*FetchExistingEtcdCertsStepSpec)
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("unexpected StepSpec type for FetchExistingEtcdCertsStep Execute: %T", currentFullSpec))
	}
	spec.PopulateDefaults()
	logger := ctx.Logger.SugaredLogger().With("host", ctx.Host.Name, "step", spec.GetName())
	res := step.NewResult(ctx, startTime, nil) // Result associated with the remote host context

	targetPKIPathVal, pkiPathOk := ctx.Task().Get(spec.TargetPKIPathSharedDataKey)
	if !pkiPathOk {
		res.Error = fmt.Errorf("target PKI path for storing certs not found in Task Cache key '%s'", spec.TargetPKIPathSharedDataKey)
		res.Status = step.StatusFailed; return res
	}
	targetPKIPath, typeOk := targetPKIPathVal.(string)
	if !typeOk || targetPKIPath == "" {
		res.Error = fmt.Errorf("invalid or empty target PKI path in Task Cache key '%s'", spec.TargetPKIPathSharedDataKey)
		res.Status = step.StatusFailed; return res
	}

	if err := os.MkdirAll(targetPKIPath, 0700); err != nil {
		res.Error = fmt.Errorf("failed to create local target PKI directory %s: %w", targetPKIPath, err)
		res.Status = step.StatusFailed; return res
	}
	logger.Infof("Ensured local PKI directory exists: %s", targetPKIPath)

	lsCmd := fmt.Sprintf("ls -p %s 2>/dev/null | grep -E '\\.pem$|\\.crt$|\\.key$' || true", spec.RemoteCertDir)
	logger.Infof("Listing remote certificate files in %s on host %s using command: %s", spec.RemoteCertDir, ctx.Host.Name, lsCmd)

	certListStr, stdErrStr, errLs := ctx.Host.Runner.RunWithOptions(ctx.GoContext, lsCmd, &connector.ExecOptions{Sudo: true, AllowFailure: true})
	if errLs != nil {
		logger.Warnf("Command '%s' to list remote certs failed or had issues (stderr: %s): %v. Proceeding cautiously.", lsCmd, stdErrStr, errLs)
	}
	if stdErrStr != "" && !strings.Contains(stdErrStr,"No such file or directory") {
		logger.Warnf("Stderr from remote list command: %s", stdErrStr)
	}

	rawFiles := strings.Split(strings.TrimSpace(certListStr), "\n")
	fetchedFileBaseNames := []string{}
	var errorsEncountered []string

	if len(rawFiles) == 1 && rawFiles[0] == "" {
		rawFiles = []string{}
	}

	logger.Infof("Found %d potential file(s) to fetch from remote directory %s.", len(rawFiles), spec.RemoteCertDir)

	for _, fileNameFromLs := range rawFiles {
		trimmedName := strings.TrimSpace(fileNameFromLs)
		if trimmedName == "" || strings.HasSuffix(trimmedName, "/") {
			continue
		}

		remoteSrc := filepath.Join(spec.RemoteCertDir, trimmedName)
		localDst := filepath.Join(targetPKIPath, trimmedName)

		logger.Infof("Fetching remote file %s to local path %s", remoteSrc, localDst)
		fetchErr := ctx.Host.Runner.Fetch(ctx.GoContext, localDst, remoteSrc)
		if fetchErr != nil {
			errMsg := fmt.Sprintf("failed to fetch %s to %s: %v", remoteSrc, localDst, fetchErr)
			logger.Warn(errMsg)
			errorsEncountered = append(errorsEncountered, errMsg)
			continue
		}
		logger.Successf("Successfully fetched %s to %s", remoteSrc, localDst)
		fetchedFileBaseNames = append(fetchedFileBaseNames, trimmedName)
	}

	ctx.Task().Set(spec.OutputFetchedFilesListKey, fetchedFileBaseNames)
	logger.Infof("List of fetched file basenames stored in Task Cache key '%s': %v", spec.OutputFetchedFilesListKey, fetchedFileBaseNames)

	if len(errorsEncountered) > 0 {
		errMsg := fmt.Sprintf("encountered %d error(s) while fetching etcd certificates: %s", len(errorsEncountered), strings.Join(errorsEncountered, "; "))
		if len(fetchedFileBaseNames) == 0 && len(rawFiles) > 0 {
			res.Error = fmt.Errorf(errMsg)
			res.Status = step.StatusFailed; return res
		}
		logger.Warn(errMsg)
	}

	if len(fetchedFileBaseNames) == 0 && len(rawFiles) > 0 && len(errorsEncountered) == len(rawFiles) {
		res.Error = fmt.Errorf("all attempts to fetch %d listed remote files failed. Errors: %s", len(rawFiles), strings.Join(errorsEncountered, "; "))
		res.Status = step.StatusFailed; return res
	}

	if len(fetchedFileBaseNames) == 0 && len(rawFiles) == 0 {
	    logger.Info("No etcd certificate files found in remote directory to fetch.")
	}

	done, checkErr := e.Check(ctx) // Pass context
	if checkErr != nil {
		res.Error = fmt.Errorf("post-execution check failed: %w", checkErr)
		res.Status = step.StatusFailed; return res
	}
	if !done {
		res.Error = fmt.Errorf("post-execution check indicates etcd certificate fetching was not fully successful or verification failed")
		res.Status = step.StatusFailed; return res
	}

	res.Message = fmt.Sprintf("Fetched %d etcd certificate files to %s.", len(fetchedFileBaseNames), targetPKIPath)
	if len(errorsEncountered) > 0 {
		res.Message += fmt.Sprintf(" Encountered %d errors during fetch.", len(errorsEncountered))
	}
	return res
}

func init() {
	step.Register(&FetchExistingEtcdCertsStepSpec{}, &FetchExistingEtcdCertsStepExecutor{})
}
