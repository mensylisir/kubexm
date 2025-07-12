package templates

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed cni/*.tmpl
//go:embed etcd/*.tmpl
//go:embed kubernetes/*.tmpl
//go:embed containerd/*.tmpl
//go:embed os/*.tmpl
var embeddedTemplates embed.FS

// Get retrieves the content of an embedded template file.
// The templateName should be the relative path from the 'templates' directory,
// e.g., "etcd/etcd.conf.yaml.tmpl" or "cni/calico.yaml.tmpl".
func Get(templateName string) (string, error) {
	// embed.FS uses 'templates' as the root for the paths specified in //go:embed
	// So, if we embed "cni/*.tmpl", a file "cni/calico.yaml.tmpl" is accessible as "cni/calico.yaml.tmpl".
	content, err := fs.ReadFile(embeddedTemplates, templateName)
	if err != nil {
		return "", fmt.Errorf("failed to read embedded template '%s': %w", templateName, err)
	}
	return string(content), nil
}

// List returns a list of all embedded template files.
func List() ([]string, error) {
	var files []string
	err := fs.WalkDir(embeddedTemplates, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk embedded templates: %w", err)
	}
	return files, nil
}
