package config

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/mensylisir/kubexm/internal/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/errors/validation"
	"github.com/mensylisir/kubexm/internal/netutil"
)

func ensureLocalHostIfEmpty(clusterConfig *v1alpha1.Cluster) error {
	if clusterConfig == nil || clusterConfig.Spec == nil {
		return nil
	}
	if len(clusterConfig.Spec.Hosts) > 0 {
		return nil
	}

	ip, err := netutil.DetectPrimaryIP()
	if err != nil {
		return fmt.Errorf("failed to detect primary host IP: %w", err)
	}

	localHost := v1alpha1.HostSpec{
		Name:    "local-1",
		Address: ip,
	}
	clusterConfig.Spec.Hosts = []v1alpha1.HostSpec{localHost}
	v1alpha1.SetDefaults_HostSpec(&clusterConfig.Spec.Hosts[0], clusterConfig)
	if clusterConfig.Spec.RoleGroups == nil {
		clusterConfig.Spec.RoleGroups = &v1alpha1.RoleGroupsSpec{}
	}
	clusterConfig.Spec.RoleGroups.Master = []string{localHost.Name}
	clusterConfig.Spec.RoleGroups.Worker = []string{localHost.Name}
	clusterConfig.Spec.RoleGroups.Etcd = []string{localHost.Name}

	return nil
}

func normalizeDeploymentTypes(clusterConfig *v1alpha1.Cluster) {
	if clusterConfig == nil || clusterConfig.Spec == nil {
		return
	}
	if clusterConfig.Spec.Etcd != nil && strings.EqualFold(clusterConfig.Spec.Etcd.Type, "exists") {
		clusterConfig.Spec.Etcd.Type = string(common.EtcdDeploymentTypeExternal)
	}
	if clusterConfig.Spec.ControlPlaneEndpoint != nil {
		cpe := clusterConfig.Spec.ControlPlaneEndpoint
		if strings.EqualFold(string(cpe.ExternalLoadBalancerType), "exists") {
			cpe.ExternalLoadBalancerType = common.ExternalLBTypeExternal
		}
		if strings.EqualFold(string(cpe.ExternalLoadBalancerType), "kubexm_kh") {
			cpe.ExternalLoadBalancerType = common.ExternalLBTypeKubexmKH
		}
		if strings.EqualFold(string(cpe.ExternalLoadBalancerType), "kubexm_kn") {
			cpe.ExternalLoadBalancerType = common.ExternalLBTypeKubexmKN
		}
		if cpe.HighAvailability != nil && cpe.HighAvailability.External != nil &&
			strings.EqualFold(cpe.HighAvailability.External.Type, "exists") {
			cpe.HighAvailability.External.Type = string(common.ExternalLBTypeExternal)
		}
		if cpe.HighAvailability != nil && cpe.HighAvailability.External != nil {
			if strings.EqualFold(cpe.HighAvailability.External.Type, "kubexm_kh") {
				cpe.HighAvailability.External.Type = string(common.ExternalLBTypeKubexmKH)
			}
			if strings.EqualFold(cpe.HighAvailability.External.Type, "kubexm_kn") {
				cpe.HighAvailability.External.Type = string(common.ExternalLBTypeKubexmKN)
			}
		}
	}
}

func filterHostValidationErrors(verrs *validation.ValidationErrors) *validation.ValidationErrors {
	if verrs == nil || verrs.IsEmpty() {
		return verrs
	}
	filtered := &validation.ValidationErrors{}
	for _, msg := range verrs.GetErrors() {
		if strings.Contains(msg, ".hosts") || strings.Contains(msg, ".roleGroups") {
			continue
		}
		filtered.Add("%s", msg)
	}
	return filtered
}

func ApplyRoleGroupsToHosts(roleGroups *v1alpha1.RoleGroupsSpec, hosts []v1alpha1.HostSpec) error {
	if roleGroups == nil || len(hosts) == 0 {
		return nil
	}

	roleMap := map[string][]string{}
	appendRoles := func(names []string, role string) {
		for _, name := range names {
			roleMap[name] = append(roleMap[name], role)
		}
	}

	appendRoles(roleGroups.Master, common.RoleMaster)
	appendRoles(roleGroups.Worker, common.RoleWorker)
	appendRoles(roleGroups.Etcd, common.RoleEtcd)
	appendRoles(roleGroups.LoadBalancer, common.RoleLoadBalancer)
	appendRoles(roleGroups.Storage, common.RoleStorage)
	appendRoles(roleGroups.Registry, common.RoleRegistry)

	for i := range hosts {
		roles := roleMap[hosts[i].Name]
		if len(roles) == 0 {
			continue
		}
		hosts[i].Roles = uniqueRoles(roles)
		hosts[i].RoleTable = make(map[string]bool, len(hosts[i].Roles))
		for _, role := range hosts[i].Roles {
			hosts[i].RoleTable[role] = true
		}
	}

	return nil
}

func uniqueRoles(roles []string) []string {
	seen := map[string]struct{}{}
	unique := make([]string, 0, len(roles))
	for _, role := range roles {
		if role == "" {
			continue
		}
		if _, ok := seen[role]; ok {
			continue
		}
		seen[role] = struct{}{}
		unique = append(unique, role)
	}
	return unique
}

func ValidateRoleGroupHosts(roleGroups *v1alpha1.RoleGroupsSpec, hosts []v1alpha1.HostSpec) error {
	validHosts := make(map[string]struct{})
	for _, host := range hosts {
		validHosts[host.Name] = struct{}{}
	}

	allHosts := []string{}
	allHosts = append(allHosts, roleGroups.Master...)
	allHosts = append(allHosts, roleGroups.Worker...)
	allHosts = append(allHosts, roleGroups.Etcd...)
	allHosts = append(allHosts, roleGroups.LoadBalancer...)
	allHosts = append(allHosts, roleGroups.Storage...)
	allHosts = append(allHosts, roleGroups.Registry...)

	for _, hostName := range allHosts {
		if _, ok := validHosts[hostName]; !ok {
			return fmt.Errorf("host '%s' from an expanded roleGroup is not defined in spec.hosts", hostName)
		}
	}
	return nil
}

func ExpandRoleGroupHosts(hosts []string) ([]string, error) {
	if hosts == nil {
		return nil, nil
	}
	expanded := make([]string, 0, len(hosts))
	for _, h := range hosts {
		currentHosts, err := ExpandHostRange(h)
		if err != nil {
			return nil, fmt.Errorf("error expanding host range '%s': %w", h, err)
		}
		expanded = append(expanded, currentHosts...)
	}
	return expanded, nil
}

func ExpandHostRange(pattern string) ([]string, error) {
	re := regexp.MustCompile(`^(.*)\[([0-9]+):([0-9]+)\](.*)$`)
	matches := re.FindStringSubmatch(pattern)

	if len(matches) == 0 {
		if strings.TrimSpace(pattern) == "" {
			return nil, errors.New("host pattern cannot be empty")
		}
		return []string{pattern}, nil
	}

	prefix := matches[1]
	startStr := matches[2]
	endStr := matches[3]
	suffix := matches[4]

	start, err := strconv.Atoi(startStr)
	if err != nil {
		return nil, fmt.Errorf("invalid start range in pattern '%s': %w", pattern, err)
	}
	end, err := strconv.Atoi(endStr)
	if err != nil {
		return nil, fmt.Errorf("invalid end range in pattern '%s': %w", pattern, err)
	}

	if start > end {
		return nil, fmt.Errorf("start range cannot be greater than end range in pattern '%s'", pattern)
	}

	var hostnames []string
	formatStr := "%s%0" + fmt.Sprintf("%dd", len(startStr)) + "%s"
	if len(startStr) == 1 || (len(startStr) > 1 && startStr[0] != '0') {
		formatStr = "%s%d%s"
	}

	for i := start; i <= end; i++ {
		hostnames = append(hostnames, fmt.Sprintf(formatStr, prefix, i, suffix))
	}

	if len(hostnames) == 0 {
		return nil, fmt.Errorf("expanded to zero hostnames for pattern '%s', check range", pattern)
	}

	return hostnames, nil
}
