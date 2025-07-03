package util

import (
	"bytes"
	"text/template"
)

// RenderTemplate executes a Go template with the given data.
func RenderTemplate(tmplStr string, data interface{}) (string, error) {
	tmpl, err := template.New("utiltemplate").Option("missingkey=error").Parse(tmplStr)
	if err != nil {
		return "", err // Error during parse (e.g., syntax error in template)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		// Error during execution (e.g., missingkey=error triggered, or type mismatch if template expects specific field types)
		return "", err
	}

	return buf.String(), nil
}
