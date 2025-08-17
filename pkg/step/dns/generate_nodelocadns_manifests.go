package dns

import (
	"bytes"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/images"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type GenerateNodeLocalDNSArtifactsStep struct {
	step.Base
	RemoteManifestPath string

	Image            string
	IP               string
	CoreDNSServiceIP string
	Corefile         string

	ExternalZones []v1alpha1.ExternalZone
	ClusterDomain string
}

type GenerateNodeLocalDNSArtifactsStepBuilder struct {
	step.Builder[GenerateNodeLocalDNSArtifactsStepBuilder, *GenerateNodeLocalDNSArtifactsStep]
}

func NewGenerateNodeLocalDNSArtifactsStepBuilder(ctx runtime.Context, instanceName string) *GenerateNodeLocalDNSArtifactsStepBuilder {
	s := &GenerateNodeLocalDNSArtifactsStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate NodeLocal DNSCache manifest", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	s.RemoteManifestPath = filepath.Join(ctx.GetUploadDir(), ctx.GetHost().GetName(), "nodelocaldns.yaml")

	clusterCfg := ctx.GetClusterConfig()

	dnsIP, err := getNthIP(clusterCfg.Spec.Network.KubeServiceCIDR, 10)
	if err != nil {
		ctx.GetLogger().Errorf("Failed to calculate CoreDNS service IP for NodeLocalDNS upstream: %v", err)
		s.CoreDNSServiceIP = ""
	} else {
		s.CoreDNSServiceIP = dnsIP.String()
	}

	imageProvider := images.NewImageProvider(&ctx)
	image := imageProvider.GetImage("k8s-dns-node-cache")
	s.Image = image.FullName()
	s.ClusterDomain = common.DefaultClusterLocal
	if clusterCfg.Spec.Kubernetes.DNSDomain != "" {
		s.ClusterDomain = clusterCfg.Spec.Kubernetes.DNSDomain
	}

	if clusterCfg.Spec.DNS != nil && clusterCfg.Spec.DNS.NodeLocalDNS != nil {
		nodeLocalDNSCfg := clusterCfg.Spec.DNS.NodeLocalDNS
		s.IP = nodeLocalDNSCfg.IP
		s.ExternalZones = nodeLocalDNSCfg.ExternalZones
	}

	b := new(GenerateNodeLocalDNSArtifactsStepBuilder).Init(s)
	return b
}

func (s *GenerateNodeLocalDNSArtifactsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *GenerateNodeLocalDNSArtifactsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	logger.Info("Rendering NodeLocalDNS Corefile from template...")
	corefileTemplateContent, err := templates.Get("dns/nodelocaldns.Corefile.tmpl")
	if err != nil {
		return fmt.Errorf("failed to get embedded nodelocaldns.Corefile.tmpl: %w", err)
	}

	corefileTmpl, err := template.New("nodelocaldnsCorefile").Funcs(template.FuncMap{
		"join": strings.Join,
	}).Parse(corefileTemplateContent)
	if err != nil {
		return fmt.Errorf("failed to parse nodelocaldns.Corefile.tmpl: %w", err)
	}

	var corefileBuffer bytes.Buffer
	if err := corefileTmpl.Execute(&corefileBuffer, s); err != nil {
		return fmt.Errorf("failed to render nodelocaldns.Corefile.tmpl: %w", err)
	}
	s.Corefile = corefileBuffer.String()

	logger.Info("Rendering main NodeLocalDNS deployment manifest...")
	mainTemplateContent, err := templates.Get("dns/nodelocaldns.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to get embedded nodelocaldns.yaml.tmpl: %w", err)
	}

	mainTmpl, err := template.New("nodelocaldnsManifest").Funcs(template.FuncMap{
		"indent": func(n int, s string) string {
			lines := strings.Split(s, "\n")
			for i, line := range lines {
				if i == len(lines)-1 && line == "" {
					continue
				}
				lines[i] = strings.Repeat(" ", n) + line
			}
			return strings.Join(lines, "\n")
		},
	}).Parse(mainTemplateContent)
	if err != nil {
		return fmt.Errorf("failed to parse nodelocaldns.yaml.tmpl: %w", err)
	}

	var finalManifestBuffer bytes.Buffer
	if err := mainTmpl.Execute(&finalManifestBuffer, s); err != nil {
		return fmt.Errorf("failed to render nodelocaldns.yaml.tmpl: %w", err)
	}

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	remoteDir := filepath.Dir(s.RemoteManifestPath)
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote dir %s: %w", remoteDir, err)
	}

	logger.Info("Uploading NodeLocalDNS manifest.", "path", s.RemoteManifestPath)
	if err := helpers.WriteContentToRemote(ctx, conn, finalManifestBuffer.String(), s.RemoteManifestPath, "0644", s.Sudo); err != nil {
		return fmt.Errorf("failed to upload NodeLocalDNS manifest: %w", err)
	}

	logger.Info("NodeLocalDNS manifest generated and uploaded successfully.")
	return nil
}

func (s *GenerateNodeLocalDNSArtifactsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	clusterCfg := ctx.GetClusterConfig()
	if clusterCfg.Spec.DNS == nil || clusterCfg.Spec.DNS.NodeLocalDNS == nil || clusterCfg.Spec.DNS.NodeLocalDNS.Enabled == nil || !*clusterCfg.Spec.DNS.NodeLocalDNS.Enabled {
		logger.Info("NodeLocalDNS is disabled, skipping artifact generation.")
		return true, nil
	}
	logger.Info("NodeLocalDNS is enabled, manifest generation will proceed if scheduled.")
	return false, nil
}

func (s *GenerateNodeLocalDNSArtifactsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Error(err, "Failed to get connector for rollback.")
		return nil
	}

	logger.Warn("Rolling back by removing remote NodeLocalDNS manifest.", "path", s.RemoteManifestPath)
	if err := runner.Remove(ctx.GoContext(), conn, s.RemoteManifestPath, s.Sudo, true); err != nil {
		logger.Error(err, "Failed to remove remote NodeLocalDNS manifest during rollback.")
	}
	return nil
}

var _ step.Step = (*GenerateNodeLocalDNSArtifactsStep)(nil)
