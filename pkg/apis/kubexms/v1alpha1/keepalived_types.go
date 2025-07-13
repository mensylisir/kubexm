package v1alpha1

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1/helpers"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/errors/validation"
	"strings"
)

type KeepalivedConfig struct {
	GlobalDefs     *GlobalDefinitions `json:"globalDefs,omitempty" yaml:"globalDefs,omitempty"`
	VRRPScripts    []VRRPScript       `json:"vrrpScripts,omitempty" yaml:"vrrpScripts,omitempty"`
	VRRPInstances  []VRRPInstance     `json:"vrrpInstances,omitempty" yaml:"vrrpInstances,omitempty"`
	VirtualServers []VirtualServer    `json:"virtualServers,omitempty" yaml:"virtualServers,omitempty"`
}

type GlobalDefinitions struct {
	RouterID          string   `json:"routerId,omitempty" yaml:"routerId,omitempty"`
	NotificationEmail []string `json:"notificationEmail,omitempty" yaml:"notificationEmail,omitempty"`
	SkipCheckAdvAddr  *bool    `json:"skipCheckAdvAddr,omitempty" yaml:"skipCheckAdvAddr,omitempty"`
	GarpInterval      *int     `json:"garpInterval,omitempty" yaml:"garpInterval,omitempty"`
	GnaInterval       *int     `json:"gnaInterval,omitempty" yaml:"gnaInterval,omitempty"`
}

type VRRPScript struct {
	Name     string `json:"name" yaml:"name"`
	Script   string `json:"script" yaml:"script"`
	Interval *int   `json:"interval,omitempty" yaml:"interval,omitempty"`
	Weight   *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
	Fall     *int   `json:"fall,omitempty" yaml:"fall,omitempty"`
	Rise     *int   `json:"rise,omitempty" yaml:"rise,omitempty"`
}

type VRRPInstance struct {
	Name            string          `json:"name" yaml:"name"`                       // e.g., "haproxy-vip"
	State           string          `json:"state,omitempty" yaml:"state,omitempty"` // "MASTER" or "BACKUP"
	Interface       string          `json:"interface" yaml:"interface"`
	VirtualRouterID int             `json:"virtualRouterId" yaml:"virtualRouterId"`
	Priority        int             `json:"priority" yaml:"priority"`
	AdvertInt       *int            `json:"advertInt,omitempty" yaml:"advertInt,omitempty"`
	Preempt         *bool           `json:"preempt,omitempty" yaml:"preempt,omitempty"`
	Auth            *Authentication `json:"authentication,omitempty" yaml:"authentication,omitempty"`
	VirtualIPs      []string        `json:"virtualIPs" yaml:"virtualIPs"` // e.g., ["172.16.0.10/24"]
	UnicastSrcIP    *string         `json:"unicastSrcIp,omitempty" yaml:"unicastSrcIp,omitempty"`
	UnicastPeers    []string        `json:"unicastPeers,omitempty" yaml:"unicastPeers,omitempty"`
	TrackScripts    []string        `json:"trackScripts,omitempty" yaml:"trackScripts,omitempty"` // e.g., ["chk_haproxy"]
}

type Authentication struct {
	AuthType string `json:"authType" yaml:"authType"`
	AuthPass string `json:"authPass" yaml:"authPass"`
}

type VirtualServer struct {
	VirtualAddress string       `json:"virtualAddress" yaml:"virtualAddress"`
	DelayLoop      *int         `json:"delayLoop,omitempty" yaml:"delayLoop,omitempty"`
	LBAlgo         string       `json:"lbAlgo" yaml:"lbAlgo"`
	LBKind         string       `json:"lbKind" yaml:"lbKind"`
	Protocol       string       `json:"protocol" yaml:"protocol"`
	RealServers    []RealServer `json:"realServers" yaml:"realServers"`
}

type RealServer struct {
	Address     string       `json:"address" yaml:"address"` // e.g., "192.168.1.1 8080"
	Weight      *int         `json:"weight,omitempty" yaml:"weight,omitempty"`
	HealthCheck *HealthCheck `json:"healthCheck,omitempty" yaml:"healthCheck,omitempty"`
}

type HealthCheck struct {
	ConnectTimeout   *int       `json:"connectTimeout,omitempty" yaml:"connectTimeout,omitempty"`
	NbGetRetry       *int       `json:"nbGetRetry,omitempty" yaml:"nbGetRetry,omitempty"`
	DelayBeforeRetry *int       `json:"delayBeforeRetry,omitempty" yaml:"delayBeforeRetry,omitempty"`
	TCPCheck         *TCPCheck  `json:"tcpCheck,omitempty" yaml:"tcpCheck,omitempty"`
	HTTPCheck        *HTTPCheck `json:"httpCheck,omitempty" yaml:"httpCheck,omitempty"`
}

type HTTPCheck struct {
	Path       string `json:"path" yaml:"path"`
	Digest     string `json:"digest,omitempty" yaml:"digest,omitempty"`
	StatusCode *int   `json:"statusCode,omitempty" yaml:"statusCode,omitempty"`
}

type TCPCheck struct {
	ConnectPort *int `json:"connectPort,omitempty" yaml:"connectPort,omitempty"`
}

func SetDefaults_KeepalivedConfig(cfg *KeepalivedConfig) {
	if cfg == nil {
		return
	}

	if cfg.GlobalDefs == nil {
		cfg.GlobalDefs = &GlobalDefinitions{}
	}
	SetDefaults_GlobalDefinitions(cfg.GlobalDefs)

	for i := range cfg.VRRPScripts {
		SetDefaults_VRRPScript(&cfg.VRRPScripts[i])
	}

	for i := range cfg.VRRPInstances {
		SetDefaults_VRRPInstance(&cfg.VRRPInstances[i])
	}

	for i := range cfg.VirtualServers {
		SetDefaults_VirtualServer(&cfg.VirtualServers[i])
	}
}

func SetDefaults_GlobalDefinitions(cfg *GlobalDefinitions) {
	if cfg.RouterID == "" {
		cfg.RouterID = common.DefaultKeepalivedRouterID
	}
	if cfg.SkipCheckAdvAddr == nil {
		cfg.SkipCheckAdvAddr = helpers.BoolPtr(true)
	}
	if cfg.GarpInterval == nil {
		cfg.GarpInterval = helpers.IntPtr(0)
	}
	if cfg.GnaInterval == nil {
		cfg.GnaInterval = helpers.IntPtr(0)
	}
	if cfg.NotificationEmail == nil {
		cfg.NotificationEmail = []string{}
	}

}

func SetDefaults_VRRPInstance(instance *VRRPInstance) {
	if instance.AdvertInt == nil {
		instance.AdvertInt = helpers.IntPtr(common.DefaultKeepalivedAdvertInt)
	}
	if instance.Preempt == nil {
		instance.Preempt = helpers.BoolPtr(common.DefaultKeepalivedPreempt)
	}
	if instance.Auth == nil {
		instance.Auth = &Authentication{
			AuthType: common.KeepalivedAuthTypePASS,
			AuthPass: common.DefaultKeepalivedAuthPass,
		}
	}
}

func SetDefaults_VirtualServer(vs *VirtualServer) {
	if vs.DelayLoop == nil {
		vs.DelayLoop = helpers.IntPtr(common.DefaultKeepalivedInterval)
	}
	if vs.LBAlgo == "" {
		vs.LBAlgo = common.DefaultKeepalivedLVSRRcheduler
	}
	if vs.LBKind == "" {
		vs.LBKind = common.DefaultKeepalivedDR
	}
	if vs.Protocol == "" {
		vs.Protocol = common.DefaultKeepalivedTCPProtocol
	}

	for i := range vs.RealServers {
		SetDefaults_RealServer(&vs.RealServers[i])
	}
}

func SetDefaults_RealServer(rs *RealServer) {
	if rs.Weight == nil {
		rs.Weight = helpers.IntPtr(common.DefaultKeepalivedWeight)
	}
	if rs.HealthCheck != nil {
		SetDefaults_HealthCheck(rs.HealthCheck)
	}
}

func SetDefaults_HealthCheck(hc *HealthCheck) {
	if hc.ConnectTimeout == nil {
		hc.ConnectTimeout = helpers.IntPtr(common.DefaultKeepalivedTimepout)
	}
	if hc.NbGetRetry == nil {
		hc.NbGetRetry = helpers.IntPtr(common.DefaultKeepalivedRetries)
	}
	if hc.DelayBeforeRetry == nil {
		hc.DelayBeforeRetry = helpers.IntPtr(common.DefaultKeepalivedRetriesInterval)
	}
	if hc.HTTPCheck != nil && hc.HTTPCheck.StatusCode == nil {
		hc.HTTPCheck.StatusCode = helpers.IntPtr(common.DefaultKeepalivedHTTPCheckSucessStatusCode)
	}
}

func SetDefaults_VRRPScript(script *VRRPScript) {
	if script.Interval == nil {
		script.Interval = helpers.IntPtr(common.DefaultKeepalivedInterval)
	}
	if script.Rise == nil {
		script.Rise = helpers.IntPtr(common.DefaultKeepalivedRise)
	}
	if script.Fall == nil {
		script.Fall = helpers.IntPtr(common.DefaultKeepalivedFall)
	}
}

func Validate_KeepalivedConfig(cfg *KeepalivedConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		verrs.Add(pathPrefix, "keepalived configuration cannot be nil")
		return
	}

	if len(cfg.VRRPInstances) == 0 && len(cfg.VirtualServers) == 0 {
		verrs.Add(pathPrefix, "at least one vrrpInstance or virtualServer must be defined")
	}

	if cfg.GlobalDefs != nil {
		Validate_GlobalDefinitions(cfg.GlobalDefs, verrs, pathPrefix+".globalDefs")
	} else {
		verrs.Add(pathPrefix, "globalDefs section is required")
	}

	// Create a set of defined script names for quick lookups
	scriptNames := make(map[string]bool)
	for i, script := range cfg.VRRPScripts {
		scriptPath := fmt.Sprintf("%s.vrrpScripts[%d]", pathPrefix, i)
		Validate_VRRPScript(&script, verrs, scriptPath)
		if helpers.IsValidNonEmptyString(script.Name) {
			scriptNames[script.Name] = true
		}
	}

	for i, instance := range cfg.VRRPInstances {
		instancePath := fmt.Sprintf("%s.vrrpInstances[%d]", pathPrefix, i)
		Validate_VRRPInstance(&instance, scriptNames, verrs, instancePath)
	}

	for i, vs := range cfg.VirtualServers {
		vsPath := fmt.Sprintf("%s.virtualServers[%d]", pathPrefix, i)
		Validate_VirtualServer(&vs, verrs, vsPath)
	}
}

func Validate_GlobalDefinitions(defs *GlobalDefinitions, verrs *validation.ValidationErrors, path string) {
	if !helpers.IsValidNonEmptyString(defs.RouterID) {
		verrs.Add(path+".routerId", "must be specified and cannot be empty")
	}
	for i, email := range defs.NotificationEmail {
		if !helpers.IsValidEmail(email) {
			verrs.Add(fmt.Sprintf("%s.notificationEmail[%d]", path, i), "invalid email format")
		}
	}
	if defs.GarpInterval != nil && *defs.GarpInterval < 0 {
		verrs.Add(path+".garpInterval", "must be a non-negative integer")
	}
	if defs.GnaInterval != nil && *defs.GnaInterval < 0 {
		verrs.Add(path+".gnaInterval", "must be a non-negative integer")
	}
}

func Validate_VRRPScript(script *VRRPScript, verrs *validation.ValidationErrors, path string) {
	if !helpers.IsValidNonEmptyString(script.Name) {
		verrs.Add(path+".name", "cannot be empty")
	}
	if !helpers.IsValidNonEmptyString(script.Script) {
		verrs.Add(path+".script", "cannot be empty")
	}
	if script.Interval != nil && !helpers.IsValidPositiveInteger(*script.Interval) {
		verrs.Add(path+".interval", "must be a positive integer")
	}
	if script.Fall != nil && !helpers.IsValidPositiveInteger(*script.Fall) {
		verrs.Add(path+".fall", "must be a positive integer")
	}
	if script.Rise != nil && !helpers.IsValidPositiveInteger(*script.Rise) {
		verrs.Add(path+".rise", "must be a positive integer")
	}
}

func Validate_VRRPInstance(instance *VRRPInstance, scriptNames map[string]bool, verrs *validation.ValidationErrors, path string) {
	if !helpers.IsValidNonEmptyString(instance.Name) {
		verrs.Add(path+".name", "cannot be empty")
	}
	if helpers.IsValidNonEmptyString(instance.State) && !helpers.ContainsString(common.ValidVRRPStates, instance.State) {
		verrs.Add(path+".state", fmt.Sprintf("must be one of %v", common.ValidVRRPStates))
	}
	if !helpers.IsValidNonEmptyString(instance.Interface) {
		verrs.Add(path+".interface", "must be specified")
	}
	if !helpers.IsValidRange(instance.VirtualRouterID, 1, 255) {
		verrs.Add(path+".virtualRouterId", fmt.Sprintf("must be between 1 and 255, got %d", instance.VirtualRouterID))
	}
	if !helpers.IsValidRange(instance.Priority, 1, 254) {
		verrs.Add(path+".priority", fmt.Sprintf("must be between 1 and 254, got %d", instance.Priority))
	}

	if instance.Auth != nil {
		authPath := path + ".authentication"
		if !helpers.ContainsString(common.ValidKeepalivedAuthTypes, instance.Auth.AuthType) {
			verrs.Add(authPath+".authType", fmt.Sprintf("invalid value '%s', must be one of %v", instance.Auth.AuthType, common.ValidKeepalivedAuthTypes))
		}
		if instance.Auth.AuthType == common.KeepalivedAuthTypePASS {
			if !helpers.IsValidNonEmptyString(instance.Auth.AuthPass) {
				verrs.Add(authPath+".authPass", "must be specified if authType is 'PASS'")
			} else if len(instance.Auth.AuthPass) > 8 {
				verrs.Add(authPath+".authPass", "password too long for some versions (max 8 chars recommended)")
			}
		}
	}

	if len(instance.VirtualIPs) == 0 {
		verrs.Add(path+".virtualIPs", "at least one virtual IP must be specified")
	}
	for i, vip := range instance.VirtualIPs {
		if !helpers.IsValidCIDR(vip) {
			verrs.Add(fmt.Sprintf("%s.virtualIPs[%d]", path, i), "invalid CIDR format")
		}
	}

	if instance.UnicastSrcIP != nil && !helpers.IsValidIP(*instance.UnicastSrcIP) {
		verrs.Add(path+".unicastSrcIp", "must be a valid IP address")
	}
	for i, peer := range instance.UnicastPeers {
		if !helpers.IsValidIP(peer) {
			verrs.Add(fmt.Sprintf("%s.unicastPeers[%d]", path, i), "must be a valid IP address")
		}
	}

	for _, scriptName := range instance.TrackScripts {
		if _, ok := scriptNames[scriptName]; !ok {
			verrs.Add(path+".trackScripts", fmt.Sprintf("script '%s' is not defined in vrrpScripts", scriptName))
		}
	}
}

func validateIPPortString(addr string, verrs *validation.ValidationErrors, path string) {
	parts := strings.Fields(addr)
	if len(parts) != 2 {
		verrs.Add(path, "must be in 'IP PORT' format")
		return
	}
	if !helpers.IsValidIP(parts[0]) {
		verrs.Add(path, fmt.Sprintf("invalid IP address part: %s", parts[0]))
	}
	if !helpers.IsValidPortString(parts[1]) {
		verrs.Add(path, fmt.Sprintf("invalid port part: %s", parts[1]))
	}
}

func Validate_VirtualServer(vs *VirtualServer, verrs *validation.ValidationErrors, path string) {
	validateIPPortString(vs.VirtualAddress, verrs, path+".virtualAddress")

	if !helpers.ContainsString(common.ValidLVSAlgos, vs.LBAlgo) {
		verrs.Add(path+".lbAlgo", fmt.Sprintf("invalid LVS scheduler '%s', must be one of %v", vs.LBAlgo, common.ValidLVSAlgos))
	}
	if !helpers.ContainsString(common.ValidLVSKinds, vs.LBKind) {
		verrs.Add(path+".lbKind", fmt.Sprintf("invalid LVS kind '%s', must be one of %v", vs.LBKind, common.ValidLVSKinds))
	}
	if !helpers.ContainsString(common.ValidProtocols, vs.Protocol) {
		verrs.Add(path+".protocol", fmt.Sprintf("invalid protocol '%s', must be one of %v", vs.Protocol, common.ValidProtocols))
	}
	if len(vs.RealServers) == 0 {
		verrs.Add(path+".realServers", "at least one real server must be specified")
	}
	for i, rs := range vs.RealServers {
		rsPath := fmt.Sprintf("%s.realServers[%d]", path, i)
		Validate_RealServer(&rs, verrs, rsPath)
	}
}

func Validate_RealServer(rs *RealServer, verrs *validation.ValidationErrors, path string) {
	validateIPPortString(rs.Address, verrs, path+".address")
	if rs.Weight != nil && !helpers.IsValidNonNegativeInteger(*rs.Weight) {
		verrs.Add(path+".weight", "must be a non-negative integer")
	}
	if rs.HealthCheck != nil {
		Validate_HealthCheck(rs.HealthCheck, verrs, path+".healthCheck")
	}
}

func Validate_HealthCheck(hc *HealthCheck, verrs *validation.ValidationErrors, path string) {
	// Enforce mutual exclusivity
	definedChecks := 0
	if hc.TCPCheck != nil {
		definedChecks++
	}
	if hc.HTTPCheck != nil {
		definedChecks++
	}
	if definedChecks > 1 {
		verrs.Add(path, "only one health check type (tcpCheck, httpCheck) can be defined at a time")
	}

	if hc.TCPCheck != nil && hc.TCPCheck.ConnectPort != nil {
		if !helpers.IsValidPort(*hc.TCPCheck.ConnectPort) {
			verrs.Add(path+".tcpCheck.connectPort", "must be a valid port (1-65535)")
		}
	}

	if hc.HTTPCheck != nil {
		if !helpers.IsValidNonEmptyString(hc.HTTPCheck.Path) {
			verrs.Add(path+".httpCheck.path", "cannot be empty")
		}
		if hc.HTTPCheck.StatusCode != nil && !helpers.IsValidRange(*hc.HTTPCheck.StatusCode, 100, 599) {
			verrs.Add(path+".httpCheck.statusCode", "must be a valid HTTP status code (100-599)")
		}
	}
}
