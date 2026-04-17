package os

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/step/helpers"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/types"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/pkg/errors"
)

const HOSTS_TEMPLATE = `
# KubeXM hosts BEGIN
{{- range .Entries }}
{{ .IP }} {{ .Hostnames }}
{{- end }}
# KubeXM hosts END
`

var _ step.Step = (*UpdateEtcHostsStep)(nil)

type HostEntry struct {
	IP        string
	Hostnames string
}

type UpdateEtcHostsStep struct {
	step.Base
	oldKubeXMBlock string
}

type UpdateEtcHostsStepBuilder struct {
	step.Builder[UpdateEtcHostsStepBuilder, *UpdateEtcHostsStep]
}

func NewUpdateEtcHostsStepBuilder(ctx runtime.ExecutionContext, instanceName string) *UpdateEtcHostsStepBuilder {
	s := &UpdateEtcHostsStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s] >> Update /etc/hosts", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(UpdateEtcHostsStepBuilder).Init(s)
	return b
}

func (s *UpdateEtcHostsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *UpdateEtcHostsStep) generateHostsEntries(ctx runtime.ExecutionContext) ([]HostEntry, error) {
	var entries []HostEntry
	entryMap := make(map[string][]string)
	cluster := ctx.GetClusterConfig()
	for _, host := range cluster.Spec.Hosts {
		ip := host.InternalAddress
		if ip == "" {
			ip = host.Address
		}

		entryMap[ip] = append(entryMap[ip], host.Name)

		if cluster.Spec.Kubernetes != nil && cluster.Spec.Kubernetes.ClusterName != "" {
			longHostname := fmt.Sprintf("%s.%s", host.Name, cluster.Spec.Kubernetes.DNSDomain)
			entryMap[ip] = append(entryMap[ip], longHostname)
		}
	}

	if cluster.Spec.Registry != nil && cluster.Spec.Registry.MirroringAndRewriting != nil {
		privateRegistry := cluster.Spec.Registry.MirroringAndRewriting.PrivateRegistry
		if privateRegistry != "" {
			registryHosts := ctx.GetHostsByRole(common.RoleRegistry)
			if len(registryHosts) == 0 {
				return nil, errors.New("registry domain is configured, but no hosts with 'registry' role found")
			}
			registryHost := registryHosts[0]
			registryIP := registryHost.GetInternalAddress()
			if registryIP == "" {
				registryIP = registryHost.GetAddress()
			}
			entryMap[registryIP] = append(entryMap[registryIP], privateRegistry)
		}
	}

	if cluster.Spec.ControlPlaneEndpoint != nil && cluster.Spec.ControlPlaneEndpoint.Domain != "" {
		domain := cluster.Spec.ControlPlaneEndpoint.Domain
		address := cluster.Spec.ControlPlaneEndpoint.Address

		if address == "" {
			address = "127.0.0.1"
		}
		entryMap[address] = append(entryMap[address], domain)
	}

	for ip, hostnames := range entryMap {
		entries = append(entries, HostEntry{
			IP:        ip,
			Hostnames: strings.Join(helpers.UniqueStrings(hostnames), " "),
		})
	}

	return entries, nil
}

func (s *UpdateEtcHostsStep) renderTemplate(ctx runtime.ExecutionContext) (string, error) {
	entries, err := s.generateHostsEntries(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to generate hosts entries: %w", err)
	}

	data := struct {
		Entries []HostEntry
	}{
		Entries: entries,
	}

	tmpl, err := template.New("hosts").Parse(HOSTS_TEMPLATE)
	if err != nil {
		return "", fmt.Errorf("failed to parse hosts template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute hosts template: %w", err)
	}

	return buf.String(), nil
}

func (s *UpdateEtcHostsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	desiredContent, err := s.renderTemplate(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to render desired hosts content")
	}

	currentContentBytes, err := runner.ReadFile(ctx.GoContext(), conn, "/etc/hosts")
	if err != nil {
		if os.IsNotExist(err) || strings.Contains(err.Error(), "No such file or directory") {
			logger.Info("/etc/hosts not found, will be created.")
			return false, nil
		}
		return false, errors.Wrap(err, "failed to read /etc/hosts on remote host")
	}
	currentContent := string(currentContentBytes)

	re := regexp.MustCompile(`(?s)# KubeXM hosts BEGIN.*# KubeXM hosts END`)
	currentBlock := re.FindString(currentContent)

	if strings.TrimSpace(currentBlock) == strings.TrimSpace(desiredContent) {
		logger.Infof("/etc/hosts is already up-to-date.")
		return true, nil
	}

	logger.Info("/etc/hosts needs to be updated.")
	return false, nil
}

func (s *UpdateEtcHostsStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())

	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "step failed"); return result, err
	}

	logger.Info("Saving current /etc/hosts state for potential rollback...")
	currentContentBytes, err := runner.ReadFile(ctx.GoContext(), conn, "/etc/hosts")
	if err != nil && !os.IsNotExist(err) && !strings.Contains(err.Error(), "No such file or directory") {
		result.MarkFailed(err, "failed to read /etc/hosts before update"); return result, err
	}
	currentContent := string(currentContentBytes)
	re := regexp.MustCompile(`(?s)# KubeXM hosts BEGIN.*# KubeXM hosts END`)
	s.oldKubeXMBlock = re.FindString(currentContent)

	baseContent := re.ReplaceAllString(currentContent, "")

	newBlock, err := s.renderTemplate(ctx)
	if err != nil {
		result.MarkFailed(err, "failed to render new hosts content"); return result, err
	}

	finalContent := strings.TrimSpace(baseContent) + "\n" + strings.TrimSpace(newBlock) + "\n"

	logger.Info("Writing new content to /etc/hosts...")
	err = helpers.WriteContentToRemote(ctx, conn, finalContent, "/etc/hosts", "0644", s.Sudo)
	if err != nil {
		result.MarkFailed(err, "failed to write to /etc/hosts"); return result, err
	}

	logger.Infof("/etc/hosts updated successfully.")
	result.MarkCompleted("step completed successfully"); return result, nil
}

func (s *UpdateEtcHostsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Attempting to roll back /etc/hosts changes...")

	currentContentBytes, err := runner.ReadFile(ctx.GoContext(), conn, "/etc/hosts")
	if err != nil && !os.IsNotExist(err) && !strings.Contains(err.Error(), "No such file or directory") {
		return errors.Wrap(err, "failed to read /etc/hosts for rollback")
	}
	currentContent := string(currentContentBytes)

	re := regexp.MustCompile(`(?s)# KubeXM hosts BEGIN.*# KubeXM hosts END`)
	baseContent := re.ReplaceAllString(currentContent, "")

	var finalContent string
	if s.oldKubeXMBlock != "" {
		finalContent = strings.TrimSpace(baseContent) + "\n" + strings.TrimSpace(s.oldKubeXMBlock) + "\n"
	} else {
		finalContent = strings.TrimSpace(baseContent) + "\n"
	}

	err = helpers.WriteContentToRemote(ctx, conn, finalContent, "/etc/hosts", "0644", s.Sudo)
	if err != nil {
		return errors.Wrapf(err, "failed to write rolled back content to /etc/hosts")
	}

	logger.Infof("/etc/hosts has been rolled back to the previous state.")
	return nil
}
