package iscsi

import "fmt"

// DetermineISCSIConfig returns the appropriate iSCSI package names and service name
// based on the provided OS ID.
func DetermineISCSIConfig(osID string) (pkgNames []string, svcName string, err error) {
	switch osID {
	case "ubuntu", "debian":
		return []string{"open-iscsi"}, "open-iscsi", nil
	case "centos", "rhel", "fedora", "almalinux", "rocky":
		return []string{"iscsi-initiator-utils"}, "iscsid", nil
	default:
		return nil, "", fmt.Errorf("unsupported OS for iSCSI client configuration: %s", osID)
	}
}
