package helm

import (
	"fmt"
	"github.com/Masterminds/semver/v3"
	"github.com/mensylisir/kubexm/pkg/common"
)

type ComponentChartBOM struct {
	KubeVersionConstraints string
	Chart                  ChartInfo
}

var componentBOMs = map[string][]ComponentChartBOM{
	string(common.CNITypeCalico): {
		{
			KubeVersionConstraints: ">= 1.31.0, < 1.34.0",
			Chart:                  ChartInfo{"tigera-operator", "https://projectcalico.docs.tigera.io/charts", "v3.30.0"},
		},
		{
			KubeVersionConstraints: ">= 1.29.0, < 1.33.0",
			Chart:                  ChartInfo{"tigera-operator", "https://projectcalico.docs.tigera.io/charts", "v3.29.0"},
		},
		{
			KubeVersionConstraints: ">= 1.27.0, < 1.31.0",
			Chart:                  ChartInfo{"tigera-operator", "https://projectcalico.docs.tigera.io/charts", "v3.28.0"},
		},
		{
			KubeVersionConstraints: ">= 1.27.0, < 1.30.0",
			Chart:                  ChartInfo{"tigera-operator", "https://projectcalico.docs.tigera.io/charts", "v3.27.3"},
		},
		{
			KubeVersionConstraints: ">= 1.24.0, < 1.29.0",
			Chart:                  ChartInfo{"tigera-operator", "https://projectcalico.docs.tigera.io/charts", "v3.26.4"},
		},
		{
			KubeVersionConstraints: "< 1.24.0",
			Chart:                  ChartInfo{"tigera-operator", "https://projectcalico.docs.tigera.io/charts", "v3.25.1"},
		},
	},
	string(common.CNITypeFlannel): {
		{
			KubeVersionConstraints: ">= 1.22.0",
			Chart:                  ChartInfo{"flannel", "https://flannel-io.github.io/flannel/", "v0.26.2"},
		},
	},
	string(common.CNITypeCilium): {
		{
			KubeVersionConstraints: ">= 1.27.0, < 1.32.0",
			Chart:                  ChartInfo{"cilium", "https://helm.cilium.io/", "1.15.7"},
		},
	},
	string(common.CNITypeKubeOvn): {
		{KubeVersionConstraints: ">= 1.29.0", Chart: ChartInfo{"kube-ovn", "https://kubeovn.github.io/kube-ovn/", "v1.15.0"}},
		{KubeVersionConstraints: ">= 1.23.0, < 1.29.0", Chart: ChartInfo{"kube-ovn", "https://kubeovn.github.io/kube-ovn/", "v1.12.0"}},
		{KubeVersionConstraints: ">= 1.20.0, < 1.23.0", Chart: ChartInfo{"kube-ovn", "https://kubeovn.github.io/kube-ovn/", "v1.10.0"}},
	},
	string(common.CNITypeHybridnet): {
		{KubeVersionConstraints: ">= 1.23.0", Chart: ChartInfo{"hybridnet", "https://alibaba.github.io/hybridnet/", "v0.8.1"}},
		{KubeVersionConstraints: "< 1.23.0, >= 1.21.0", Chart: ChartInfo{"hybridnet", "https://alibaba.github.io/hybridnet/", "v0.6.1"}},
	},
	string(common.CNITypeMultus): {
		{KubeVersionConstraints: ">= 1.20.0", Chart: ChartInfo{"multus", "https://k8s-at-home.com/charts/", "5.0.1"}},
	},

	"ingress-nginx": {
		{
			KubeVersionConstraints: ">= 1.29.0, < 1.34.0",
			Chart:                  ChartInfo{"ingress-nginx", "https://kubernetes.github.io/ingress-nginx/", "4.13.0"},
		},
		{
			KubeVersionConstraints: ">= 1.28.0, < 1.33.0",
			Chart:                  ChartInfo{"ingress-nginx", "https://kubernetes.github.io/ingress-nginx/", "4.12.4"},
		},
		{
			KubeVersionConstraints: ">= 1.26.0, < 1.31.0",
			Chart:                  ChartInfo{"ingress-nginx", "https://kubernetes.github.io/ingress-nginx/", "4.11.8"},
		},
		{
			KubeVersionConstraints: ">= 1.26.0, < 1.31.0",
			Chart:                  ChartInfo{"ingress-nginx", "https://kubernetes.github.io/ingress-nginx/", "4.10.6"},
		},
		{
			KubeVersionConstraints: ">= 1.25.0, < 1.30.0",
			Chart:                  ChartInfo{"ingress-nginx", "https://kubernetes.github.io/ingress-nginx/", "4.9.1"},
		},
	},

	"longhorn": {
		{
			KubeVersionConstraints: ">= 1.25.0",
			Chart:                  ChartInfo{"longhorn", "https://charts.longhorn.io", "1.9.0"},
		},
		{
			KubeVersionConstraints: ">= 1.25.0",
			Chart:                  ChartInfo{"longhorn", "https://charts.longhorn.io", "1.8.2"},
		},
		{
			KubeVersionConstraints: ">= 1.21.0, < 1.25.0",
			Chart:                  ChartInfo{"longhorn", "https://charts.longhorn.io", "1.7.3"},
		},
	},
	"openebs": {
		{
			KubeVersionConstraints: ">= 1.25.0",
			Chart:                  ChartInfo{"openebs", "https://openebs.github.io/charts", "3.3.0"},
		},
	},
	"nfs-subdir-external-provisioner": {
		{
			KubeVersionConstraints: ">= 1.25.0",
			Chart:                  ChartInfo{"nfs-subdir-external-provisioner", "https://kubernetes-sigs.github.io/nfs-subdir-external-provisioner/", "4.0.18"},
		},
	},
	"csi-driver-nfs": {
		{
			KubeVersionConstraints: ">= 1.25.0",
			Chart:                  ChartInfo{"csi-driver-nfs", "https://raw.githubusercontent.com/kubernetes-csi/csi-driver-nfs/master/charts", "4.11.0"},
		},
	},
	"argocd": {
		{
			KubeVersionConstraints: ">= 1.24.0",
			Chart:                  ChartInfo{"argo-cd", "https://argoproj.github.io/argo-helm", "6.11.1"},
		},
	},
}

func GetChartInfo(componentName string, kubeVersionStr string) *ChartInfo {
	if componentName == "" || kubeVersionStr == "" {
		return nil
	}

	k8sVersion, err := semver.NewVersion(kubeVersionStr)
	if err != nil {
		fmt.Printf("Warning: could not parse kubernetes version '%s': %v. Unable to find a compatible chart.\n", kubeVersionStr, err)
		return nil
	}

	componentBOMList, ok := componentBOMs[componentName]
	if !ok {
		fmt.Printf("Warning: no chart BOM found for component '%s'\n", componentName)
		return nil
	}

	var candidates []ComponentChartBOM
	for _, bomEntry := range componentBOMList {
		constraints, err := semver.NewConstraint(bomEntry.KubeVersionConstraints)
		if err != nil {
			fmt.Printf("Error: invalid version constraint in BOM for %s: '%s'. Skipping this entry.\n", componentName, bomEntry.KubeVersionConstraints)
			continue
		}

		if constraints.Check(k8sVersion) {
			candidates = append(candidates, bomEntry)
		}
	}

	if len(candidates) == 0 {
		fmt.Printf("Warning: no compatible chart version found in BOM for component '%s' with Kubernetes version '%s'\n", componentName, kubeVersionStr)
		return nil
	}

	bestCandidate := candidates[0]
	bestVersion, err := semver.NewVersion(bestCandidate.Chart.Version)
	if err != nil {
		fmt.Printf("Warning: could not parse chart version '%s' for component '%s'. Using it as initial best guess.\n", bestCandidate.Chart.Version, componentName)
	}

	for i := 1; i < len(candidates); i++ {
		currentVersion, err := semver.NewVersion(candidates[i].Chart.Version)
		if err != nil {
			fmt.Printf("Warning: could not parse chart version '%s' for component '%s'. Skipping for comparison.\n", candidates[i].Chart.Version, componentName)
			continue
		}

		if bestVersion == nil || currentVersion.GreaterThan(bestVersion) {
			bestVersion = currentVersion
			bestCandidate = candidates[i]
		}
	}

	chartCopy := bestCandidate.Chart
	return &chartCopy
}

func GetManagedChartNames() []string {
	names := make([]string, 0, len(componentBOMs))
	for name := range componentBOMs {
		names = append(names, name)
	}
	return names
}
