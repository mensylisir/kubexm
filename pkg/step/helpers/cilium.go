package helpers

import (
	"github.com/mensylisir/kubexm/pkg/runtime"
)

func DecideCiliumChartSource(ctx runtime.Context) *ChartSourceDecision {
	decision := &ChartSourceDecision{}
	clusterCfg := ctx.GetClusterConfig()
	logger := ctx.GetLogger()

	decision.RepoName = "cilium"
	decision.RepoURL = "https://helm.cilium.io/"
	decision.ChartName = "cilium"
	decision.Version = ""

	if clusterCfg.Spec.HelmRepo != nil && clusterCfg.Spec.HelmRepo.Repo != "" {
		logger.Infof("Using global Helm repository for Cilium: %s", clusterCfg.Spec.HelmRepo.Repo)
		globalHelmRepo := clusterCfg.Spec.HelmRepo

		decision.RepoURL = globalHelmRepo.Repo
		decision.RepoName = globalHelmRepo.Name
	}

	if clusterCfg.Spec.Network.Cilium != nil && clusterCfg.Spec.Network.Cilium.Source.Chart != nil && clusterCfg.Spec.Network.Cilium.Source.Chart.Name != "" {
		logger.Info("Using user-defined escape hatch for Cilium chart source.")
		userChart := clusterCfg.Spec.Network.Cilium.Source.Chart

		if userChart.Repo != "" {
			decision.RepoURL = userChart.Repo
		}
		decision.RepoName = userChart.Name
		decision.ChartName = userChart.Name
		decision.Version = userChart.Version
	}

	logger.Infof(
		"Final Cilium chart source decision: RepoName=[%s], RepoURL=[%s], ChartName=[%s], Version=[%s]",
		decision.RepoName,
		decision.RepoURL,
		decision.ChartName,
		decision.Version,
	)

	return decision
}
