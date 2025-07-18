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

func Get(templateName string) (string, error) {
	content, err := fs.ReadFile(embeddedTemplates, templateName)
	if err != nil {
		return "", fmt.Errorf("failed to read embedded template '%s': %w", templateName, err)
	}
	return string(content), nil
}

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
