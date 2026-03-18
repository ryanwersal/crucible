package script

import (
	"os"
	"os/exec"
	"strings"
	"text/template"
)

// templateFuncMap returns the built-in template functions for dotfile templating.
// Function names follow Sprig conventions for future migration compatibility.
func templateFuncMap() template.FuncMap {
	return template.FuncMap{
		"env": os.Getenv,
		"lookPath": func(name string) string {
			p, _ := exec.LookPath(name)
			return p
		},
		"default": func(fallback, val any) any {
			if val == nil {
				return fallback
			}
			if s, ok := val.(string); ok && s == "" {
				return fallback
			}
			return val
		},
		"hasPrefix": strings.HasPrefix,
		"hasSuffix": strings.HasSuffix,
		"contains":  strings.Contains,
		"replace": func(old, new, s string) string {
			return strings.ReplaceAll(s, old, new)
		},
		"lower":     strings.ToLower,
		"upper":     strings.ToUpper,
		"trimSpace": strings.TrimSpace,
		"join": func(sep string, elems []string) string {
			return strings.Join(elems, sep)
		},
	}
}
