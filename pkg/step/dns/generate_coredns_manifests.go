package dns

import (
	"bytes"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/images"
	"net"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type GenerateCoreDNSArtifactsStep struct {
	step.Base
	RemoteManifestPath string

	ClusterIP   string
	Image       string
	Replicas    int
	DNSEtcHosts string
	Corefile    string

	CoreDNSConfig *v1alpha1.CoreDNS
	ClusterDomain string
}

type GenerateCoreDNSArtifactsStepBuilder struct {
	step.Builder[GenerateCoreDNSArtifactsStepBuilder, *GenerateCoreDNSArtifactsStep]
}

func NewGenerateCoreDNSArtifactsStepBuilder(ctx runtime.Context, instanceName string) *GenerateCoreDNSArtifactsStepBuilder {
	s := &GenerateCoreDNSArtifactsStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate CoreDNS manifest", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	s.RemoteManifestPath = filepath.Join(ctx.GetUploadDir(), ctx.GetHost().GetName(), "coredns.yaml")

	clusterCfg := ctx.GetClusterConfig()

	dnsIP, err := getNthIP(clusterCfg.Spec.Network.KubeServiceCIDR, 10)
	if err != nil {
		ctx.GetLogger().Errorf("Failed to calculate CoreDNS service IP: %v", err)
		s.ClusterIP = ""
	} else {
		s.ClusterIP = dnsIP.String()
	}

	s.Replicas = 2
	s.ClusterDomain = common.DefaultClusterLocal
	if clusterCfg.Spec.Kubernetes.DNSDomain != "" {
		s.ClusterDomain = clusterCfg.Spec.Kubernetes.DNSDomain
	}
	imageProvider := images.NewImageProvider(&ctx)
	image := imageProvider.GetImage("coredns")
	s.Image = image.FullName()
	if clusterCfg.Spec.DNS != nil {
		s.DNSEtcHosts = clusterCfg.Spec.DNS.DNSEtcHosts
		s.CoreDNSConfig = clusterCfg.Spec.DNS.CoreDNS
		if s.CoreDNSConfig == nil {
			s.CoreDNSConfig = &v1alpha1.CoreDNS{}
		}
	} else {
		s.CoreDNSConfig = &v1alpha1.CoreDNS{}
	}

	b := new(GenerateCoreDNSArtifactsStepBuilder).Init(s)
	return b
}

func getNthIP(cidrStr string, n int) (net.IP, error) {
	firstCIDR := strings.Split(cidrStr, ",")[0]

	ip, ipNet, err := net.ParseCIDR(firstCIDR)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR string %q: %w", firstCIDR, err)
	}

	ipv4 := ip.To4()
	if ipv4 == nil {
		return nil, fmt.Errorf("only IPv4 is supported for IP calculation, but got %q", firstCIDR)
	}

	ipUint := uint32(ipv4[0])<<24 | uint32(ipv4[1])<<16 | uint32(ipv4[2])<<8 | uint32(ipv4[3])

	ipUint += uint32(n)

	resultIP := make(net.IP, 4)
	resultIP[0] = byte(ipUint >> 24)
	resultIP[1] = byte(ipUint >> 16)
	resultIP[2] = byte(ipUint >> 8)
	resultIP[3] = byte(ipUint)

	if !ipNet.Contains(resultIP) {
		return nil, fmt.Errorf("calculated IP %s is outside the subnet %s", resultIP, ipNet)
	}

	return resultIP, nil
}

func (s *GenerateCoreDNSArtifactsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *GenerateCoreDNSArtifactsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Manifest generation step will always run if scheduled to ensure it's up-to-date.")
	return false, nil
}

func (s *GenerateCoreDNSArtifactsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	logger.Info("Rendering Corefile from template...")
	corefileTemplateContent, err := templates.Get("dns/Corefile.tmpl")
	if err != nil {
		return fmt.Errorf("failed to get embedded Corefile.tmpl: %w", err)
	}

	corefileTmpl, err := template.New("Corefile").Funcs(template.FuncMap{
		"join": strings.Join,
	}).Parse(corefileTemplateContent)
	if err != nil {
		return fmt.Errorf("failed to parse Corefile.tmpl: %w", err)
	}

	var corefileBuffer bytes.Buffer
	if err := corefileTmpl.Execute(&corefileBuffer, s); err != nil {
		return fmt.Errorf("failed to render Corefile.tmpl: %w", err)
	}
	s.Corefile = corefileBuffer.String()

	logger.Info("Rendering main CoreDNS deployment manifest...")
	mainTemplateContent, err := templates.Get("dns/coredns.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to get embedded coredns.yaml.tmpl: %w", err)
	}

	mainTmpl, err := template.New("corednsManifest").Funcs(template.FuncMap{
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
		return fmt.Errorf("failed to parse coredns.yaml.tmpl: %w", err)
	}

	var finalManifestBuffer bytes.Buffer
	if err := mainTmpl.Execute(&finalManifestBuffer, s); err != nil {
		return fmt.Errorf("failed to render coredns.yaml.tmpl: %w", err)
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

	logger.Info("Uploading CoreDNS manifest.", "path", s.RemoteManifestPath)
	if err := helpers.WriteContentToRemote(ctx, conn, finalManifestBuffer.String(), s.RemoteManifestPath, "0644", s.Sudo); err != nil {
		return fmt.Errorf("failed to upload CoreDNS manifest: %w", err)
	}

	logger.Info("CoreDNS manifest generated and uploaded successfully.")
	return nil
}

func (s *GenerateCoreDNSArtifactsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Error(err, "Failed to get connector for rollback.")
		return nil
	}

	logger.Warn("Rolling back by removing remote CoreDNS manifest.", "path", s.RemoteManifestPath)
	if err := runner.Remove(ctx.GoContext(), conn, s.RemoteManifestPath, s.Sudo, true); err != nil {
		logger.Error(err, "Failed to remove remote CoreDNS manifest during rollback.")
	}
	return nil
}

var _ step.Step = (*GenerateCoreDNSArtifactsStep)(nil)
