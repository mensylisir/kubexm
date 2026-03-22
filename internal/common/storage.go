package common

const (
	DefaultStorageClass                      = "openebs-loal"
	DefaultOpenEBSConfigEnabled              = true
	DefaultOpenEBSConfigVersion              = "2.8"
	DefaultDefaultOpenEBSConfigEngines       = "LocalHostpath"
	DefaultMayastorEngineConfig              = false
	DefaultJivaEngineConfig                  = false
	DefaultCStorEngineConfig                 = false
	DefaultLocalHostpathEngineConfigEnabled  = true
	DefaultLocalHostpathEngineConfigBasePath = "/var/openebs/local"
	DefaultNFSConfigEnabled                  = false
	DefaultNFSConfigStorageClassName         = "nfs-client"
	DefaultRookCephConfigEnabled             = false
	DefaultRookCephConfigVersion             = "2.9"
	DefaultRookCephConfigUseAllDevices       = true
	DefaultRookCephConfigCreateBlockPool     = true
	DefaultRookCephConfigCreateFilesystem    = true
	DefaultLonghornConfigEnabled             = false
	DefaultLonghornConfigVersion             = "v2.7.2"
	DefaultLonghornConfigDefaultDataPath     = "/var/lib/longhorn"
	StorageComponentOpenEBS                  = "openebs"
	StorageComponentNFS                      = "nfs"
	StorageComponentRookCeph                 = "rook-ceph"
	StorageComponentLonghorn                 = "longhorn"
)
