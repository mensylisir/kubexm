package helpers

import (
	"github.com/mensylisir/kubexm/pkg/runtime"
)

type ChartSourceDecision struct {
	RepoURL   string
	RepoName  string
	ChartName string
	Version   string
}

func DecideCalicoChartSource(ctx runtime.Context) *ChartSourceDecision {
	decision := &ChartSourceDecision{}
	clusterCfg := ctx.GetClusterConfig()
	userCalicoCfg := clusterCfg.Spec.Network.Calico

	decision.RepoName = "projectcalico"
	decision.RepoURL = "https://docs.tigera.io/calico/charts"
	decision.ChartName = "tigera-operator"

	if userCalicoCfg != nil && userCalicoCfg.Source.Chart != nil && userCalicoCfg.Source.Chart.Name != "" {
		userChart := userCalicoCfg.Source.Chart
		decision.RepoURL = userChart.Repo
		decision.RepoName = userChart.Name
		decision.ChartName = userChart.Name
		decision.Version = userChart.Version
		return decision
	}

	if clusterCfg.Spec.HelmRepo != nil && clusterCfg.Spec.HelmRepo.Repo != "" {
		globalHelmRepo := clusterCfg.Spec.HelmRepo
		decision.RepoURL = globalHelmRepo.Repo
		decision.RepoName = globalHelmRepo.Name
		return decision
	}

	return decision
}
