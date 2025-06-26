package common

import "testing"

func TestGeneralConstants(t *testing.T) {
	t.Run("OperationalConstants", func(t *testing.T) {
		if KUBEXM != ".kubexm" {
			t.Errorf("KUBEXM constant is incorrect: got %s, want .kubexm", KUBEXM)
		}
		if DefaultLogsDir != "logs" {
			t.Errorf("DefaultLogsDir constant is incorrect: got %s, want logs", DefaultLogsDir)
		}
		if ControlNodeHostName != "kubexm-control-node" {
			t.Errorf("ControlNodeHostName constant is incorrect: got %s, want kubexm-control-node", ControlNodeHostName)
		}
		if ControlNodeRole != "control-node" {
			t.Errorf("ControlNodeRole constant is incorrect: got %s, want control-node", ControlNodeRole)
		}
	})

	t.Run("StatusConstants", func(t *testing.T) {
		if StatusPending != "Pending" {
			t.Errorf("StatusPending constant is incorrect: got %s, want Pending", StatusPending)
		}
		if StatusProcessing != "Processing" {
			t.Errorf("StatusProcessing constant is incorrect: got %s, want Processing", StatusProcessing)
		}
		if StatusSuccess != "Success" {
			t.Errorf("StatusSuccess constant is incorrect: got %s, want Success", StatusSuccess)
		}
		if StatusFailed != "Failed" {
			t.Errorf("StatusFailed constant is incorrect: got %s, want Failed", StatusFailed)
		}
	})

	t.Run("NodeConditionConstants", func(t *testing.T) {
		if NodeConditionReady != "Ready" {
			t.Errorf("NodeConditionReady constant is incorrect: got %s, want Ready", NodeConditionReady)
		}
	})

	t.Run("CNIPluginNames", func(t *testing.T) {
		if CNICalico != "calico" {
			t.Errorf("CNICalico constant is incorrect: got %s, want calico", CNICalico)
		}
		if CNIFlannel != "flannel" {
			t.Errorf("CNIFlannel constant is incorrect: got %s, want flannel", CNIFlannel)
		}
		if CNICilium != "cilium" {
			t.Errorf("CNICilium constant is incorrect: got %s, want cilium", CNICilium)
		}
		if CNIMultus != "multus" {
			t.Errorf("CNIMultus constant is incorrect: got %s, want multus", CNIMultus)
		}
	})

	t.Run("CacheKeyConstants", func(t *testing.T) {
		if CacheKeyHostFactsPrefix != "facts.host." {
			t.Errorf("CacheKeyHostFactsPrefix constant is incorrect: got %s, want facts.host.", CacheKeyHostFactsPrefix)
		}
		if CacheKeyClusterCACert != "pki.ca.cert" {
			t.Errorf("CacheKeyClusterCACert constant is incorrect: got %s, want pki.ca.cert", CacheKeyClusterCACert)
		}
		if CacheKeyClusterCAKey != "pki.ca.key" {
			t.Errorf("CacheKeyClusterCAKey constant is incorrect: got %s, want pki.ca.key", CacheKeyClusterCAKey)
		}
	})
}
