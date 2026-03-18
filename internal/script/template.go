package script

import (
	"bytes"
	"text/template"
)

// renderTemplate executes a Go text/template with the given data and returns
// the rendered content.
func renderTemplate(name, tmplContent string, data map[string]any) ([]byte, error) {
	tmpl, err := template.New(name).Funcs(templateFuncMap()).Parse(tmplContent)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
