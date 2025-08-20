package preflight

import (
	"fmt"
	"net"
	"regexp"
	"time"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type LintClusterSpecStep struct {
	step.Base
	clusterSpec *v1alpha1.ClusterSpec
}

type LintClusterSpecStepBuilder struct {
	step.Builder[LintClusterSpecStepBuilder, *LintClusterSpecStep]
}

func NewLintClusterSpecStepBuilder(ctx runtime.Context, instanceName string) *LintClusterSpecStepBuilder {
	s := &LintClusterSpecStep{
		clusterSpec: ctx.GetClusterConfig().Spec,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Statically lint and validate the Cluster configuration file"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(LintClusterSpecStepBuilder).Init(s)
	return b
}

func (s *LintClusterSpecStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *LintClusterSpecStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Checking if Cluster configuration file is valid...")
	return false, nil
}

func (s *LintClusterSpecStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	logger.Info("Linting Cluster configuration...")

	var validationErrors []string

	if s.clusterSpec.Network != nil {
		if errs := s.validateCIDR("PodCIDR", s.clusterSpec.Network.KubePodsCIDR); len(errs) > 0 {
			validationErrors = append(validationErrors, errs...)
		}
		if errs := s.validateCIDR("ServiceCIDR", s.clusterSpec.Network.KubeServiceCIDR); len(errs) > 0 {
			validationErrors = append(validationErrors, errs...)
		}
		if err := s.validateCIDROverlap(); err != nil {
			validationErrors = append(validationErrors, err.Error())
		}
	}

	if errs := s.validateHostsAndRoles(); len(errs) > 0 {
		validationErrors = append(validationErrors, errs...)
	}

	if s.clusterSpec.ControlPlaneEndpoint != nil {
		if s.clusterSpec.ControlPlaneEndpoint.Address == "" && s.clusterSpec.ControlPlaneEndpoint.Domain == "" {
			validationErrors = append(validationErrors, "ControlPlaneEndpoint requires either 'address' or 'domain' to be set")
		}
	} else {
		validationErrors = append(validationErrors, "ControlPlaneEndpoint is not defined")
	}

	if s.clusterSpec.Kubernetes != nil {
		versionRegex := regexp.MustCompile(`^v\d+\.\d+\.\d+$`)
		if !versionRegex.MatchString(s.clusterSpec.Kubernetes.Version) {
			validationErrors = append(validationErrors, fmt.Sprintf("Kubernetes version '%s' is not in a valid format (e.g., v1.25.3)", s.clusterSpec.Kubernetes.Version))
		}
	}

	if len(validationErrors) > 0 {
		errorMsg := "Cluster configuration validation failed with the following errors:\n"
		for _, e := range validationErrors {
			errorMsg += fmt.Sprintf("- %s\n", e)
		}
		return fmt.Errorf(errorMsg)
	}

	logger.Info("Cluster configuration linting passed successfully.")
	return nil
}

func (s *LintClusterSpecStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("No action to roll back for a check-only step.")
	return nil
}

func (s *LintClusterSpecStep) validateCIDR(name, cidr string) []string {
	var errs []string
	if cidr == "" {
		errs = append(errs, fmt.Sprintf("%s is not defined", name))
		return errs
	}
	_, _, err := net.ParseCIDR(cidr)
	if err != nil {
		errs = append(errs, fmt.Sprintf("invalid %s '%s': %v", name, cidr, err))
	}
	return errs
}

func (s *LintClusterSpecStep) validateCIDROverlap() error {
	podCIDR := s.clusterSpec.Network.KubePodsCIDR
	svcCIDR := s.clusterSpec.Network.KubeServiceCIDR
	if podCIDR == "" || svcCIDR == "" {
		return nil
	}

	_, podNet, err1 := net.ParseCIDR(podCIDR)
	_, svcNet, err2 := net.ParseCIDR(svcCIDR)
	if err1 != nil || err2 != nil {
		return nil
	}

	if podNet.Contains(svcNet.IP) || svcNet.Contains(podNet.IP) {
		return fmt.Errorf("PodCIDR (%s) and ServiceCIDR (%s) overlap", podCIDR, svcCIDR)
	}
	return nil
}

func (s *LintClusterSpecStep) validateHostsAndRoles() []string {
	var errs []string
	if len(s.clusterSpec.Hosts) == 0 {
		errs = append(errs, "no hosts are defined in the cluster spec")
		return errs
	}

	hostNames := make(map[string]bool)
	hostAddresses := make(map[string]bool)
	for _, host := range s.clusterSpec.Hosts {
		if host.Name == "" {
			errs = append(errs, fmt.Sprintf("a host is defined with an empty name (address: %s)", host.Address))
		}
		if host.Address == "" {
			errs = append(errs, fmt.Sprintf("host '%s' has an empty address", host.Name))
		}

		if host.Address != "" {
			if hostAddresses[host.Address] {
				errs = append(errs, fmt.Sprintf("duplicate host address '%s' found in spec", host.Address))
			}
			hostAddresses[host.Address] = true
		}

		if hostNames[host.Name] {
			errs = append(errs, fmt.Sprintf("duplicate host name '%s' found in spec", host.Name))
		}
		hostNames[host.Name] = true
	}

	if s.clusterSpec.RoleGroups == nil || len(s.clusterSpec.RoleGroups.Master) == 0 {
		errs = append(errs, "at least one master node must be defined in roleGroups")
	}

	if s.clusterSpec.RoleGroups != nil {
		allRoles := [][]string{
			s.clusterSpec.RoleGroups.Master,
			s.clusterSpec.RoleGroups.Worker,
			s.clusterSpec.RoleGroups.Etcd,
			s.clusterSpec.RoleGroups.LoadBalancer,
			// ... (add other roles)
		}
		roleNames := []string{"master", "worker", "etcd", "loadbalancer"}

		for i, roleGroup := range allRoles {
			for _, hostName := range roleGroup {
				if !hostNames[hostName] {
					errs = append(errs, fmt.Sprintf("%s role assigned to non-existent host '%s'", roleNames[i], hostName))
				}
			}
		}

		if len(s.clusterSpec.RoleGroups.Master) > 1 && len(s.clusterSpec.RoleGroups.Master)%2 == 0 {
			errs = append(errs, fmt.Sprintf("for high-availability, the number of master nodes should be odd, but got %d", len(s.clusterSpec.RoleGroups.Master)))
		}

		if len(s.clusterSpec.RoleGroups.Etcd) > 1 && len(s.clusterSpec.RoleGroups.Etcd)%2 == 0 {
			errs = append(errs, fmt.Sprintf("for a stable etcd cluster, the number of etcd nodes should be odd, but got %d", len(s.clusterSpec.RoleGroups.Etcd)))
		}
	}

	return errs
}

var _ step.Step = (*LintClusterSpecStep)(nil)
