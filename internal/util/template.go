package util

import (
	"bytes"
	"fmt"
	"github.com/Masterminds/sprig/v3"
	"text/template"
)

func RenderTemplate(tmplStr string, data interface{}) (string, error) {
	tmpl, err := template.New("utiltemplate").
		Funcs(sprig.TxtFuncMap()).
		Option("missingkey=error").
		Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}
