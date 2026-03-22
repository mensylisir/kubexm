package os

// OSBOMProvider is an interface for providing OS-specific package information
type OSBOMProvider interface {
	// GetComponentPackages returns the required packages for a component on a specific OS
	GetComponentPackages(component OSComponent, osType OSType, osVersion OSVersion) ([]OSPackage, error)
	
	// GetOSPackages returns all packages for a specific OS
	GetOSPackages(osType OSType, osVersion OSVersion) ([]OSPackage, error)
	
	// IsComponentSupported checks if a component is supported on a specific OS
	IsComponentSupported(component OSComponent, osType OSType, osVersion OSVersion) bool
}

// DefaultOSBOMProvider implements the OSBOMProvider interface
type DefaultOSBOMProvider struct{}

// NewDefaultOSBOMProvider creates a new DefaultOSBOMProvider
func NewDefaultOSBOMProvider() *DefaultOSBOMProvider {
	return &DefaultOSBOMProvider{}
}

// GetComponentPackages returns the required packages for a component on a specific OS
func (p *DefaultOSBOMProvider) GetComponentPackages(component OSComponent, osType OSType, osVersion OSVersion) ([]OSPackage, error) {
	bom := GetComponentOSPkgBOM(component, osType, osVersion)
	if bom == nil {
		return nil, nil
	}
	return bom.Packages, nil
}

// GetOSPackages returns all packages for a specific OS
func (p *DefaultOSBOMProvider) GetOSPackages(osType OSType, osVersion OSVersion) ([]OSPackage, error) {
	var allPackages []OSPackage
	
	// Get packages for all components on this OS
	for component := range osComponentBOMs {
		packages, err := p.GetComponentPackages(component, osType, osVersion)
		if err != nil {
			return nil, err
		}
		allPackages = append(allPackages, packages...)
	}
	
	// Remove duplicates
	uniquePackages := make(map[string]OSPackage)
	for _, pkg := range allPackages {
		uniquePackages[pkg.Name] = pkg
	}
	
	packages := make([]OSPackage, 0, len(uniquePackages))
	for _, pkg := range uniquePackages {
		packages = append(packages, pkg)
	}
	
	return packages, nil
}

// IsComponentSupported checks if a component is supported on a specific OS
func (p *DefaultOSBOMProvider) IsComponentSupported(component OSComponent, osType OSType, osVersion OSVersion) bool {
	return GetComponentOSPkgBOM(component, osType, osVersion) != nil
}