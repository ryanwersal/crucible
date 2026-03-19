package script

import "sort"

// TemplateFuncNames returns a sorted list of all built-in template function names.
func TemplateFuncNames() []string {
	m := templateFuncMap()
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
