package script

import (
	"bytes"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"text/template"
)

func TestTemplateFuncMap(t *testing.T) {
	t.Parallel()

	funcs := templateFuncMap()

	tests := []struct {
		name     string
		tmpl     string
		data     map[string]any
		setup    func(t *testing.T)
		expected string
	}{
		{
			name:     "env reads environment variable",
			tmpl:     `{{ env "HOME" }}`,
			expected: os.Getenv("HOME"),
		},
		{
			name:     "env returns empty for missing var",
			tmpl:     `{{ env "CRUCIBLE_TEST_NONEXISTENT_VAR" }}`,
			expected: "",
		},
		{
			name:     "lookPath finds existing command",
			tmpl:     `{{ if lookPath "go" }}found{{ end }}`,
			expected: "found",
		},
		{
			name:     "lookPath returns empty for missing command",
			tmpl:     `{{ lookPath "crucible_nonexistent_cmd_xyz" }}`,
			expected: "",
		},
		{
			name:     "default with empty string",
			tmpl:     `{{ "" | default "fallback" }}`,
			expected: "fallback",
		},
		{
			name:     "default with non-empty string",
			tmpl:     `{{ "actual" | default "fallback" }}`,
			expected: "actual",
		},
		{
			name:     "default with nil",
			tmpl:     `{{ .missing | default "fallback" }}`,
			data:     map[string]any{},
			expected: "fallback",
		},
		{
			name:     "hasPrefix true",
			tmpl:     `{{ hasPrefix "darwin" "dar" }}`,
			expected: "true",
		},
		{
			name:     "hasPrefix false",
			tmpl:     `{{ hasPrefix "linux" "dar" }}`,
			expected: "false",
		},
		{
			name:     "hasSuffix true",
			tmpl:     `{{ hasSuffix "host.local" ".local" }}`,
			expected: "true",
		},
		{
			name:     "hasSuffix false",
			tmpl:     `{{ hasSuffix "host.com" ".local" }}`,
			expected: "false",
		},
		{
			name:     "contains true",
			tmpl:     `{{ contains "arm64" "arm" }}`,
			expected: "true",
		},
		{
			name:     "contains false",
			tmpl:     `{{ contains "x86_64" "arm" }}`,
			expected: "false",
		},
		{
			name:     "replace",
			tmpl:     `{{ replace "darwin" "macOS" "darwin is great" }}`,
			expected: "macOS is great",
		},
		{
			name:     "lower",
			tmpl:     `{{ "HELLO" | lower }}`,
			expected: "hello",
		},
		{
			name:     "upper",
			tmpl:     `{{ "hello" | upper }}`,
			expected: "HELLO",
		},
		{
			name:     "trimSpace",
			tmpl:     `{{ "  hello  " | trimSpace }}`,
			expected: "hello",
		},
		{
			name:     "join",
			tmpl:     `{{ join "," .items }}`,
			data:     map[string]any{"items": []string{"a", "b", "c"}},
			expected: "a,b,c",
		},
		{
			name:     "env with default pipeline",
			tmpl:     `{{ env "CRUCIBLE_TEST_NONEXISTENT_VAR" | default "vim" }}`,
			expected: "vim",
		},
		{
			name:     "lookPath in conditional",
			tmpl:     `{{ if lookPath "go" }}yes{{ else }}no{{ end }}`,
			expected: "yes",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.setup != nil {
				tc.setup(t)
			}

			tmpl, err := template.New("test").Funcs(funcs).Parse(tc.tmpl)
			if err != nil {
				t.Fatalf("parse template: %v", err)
			}

			var buf bytes.Buffer
			data := tc.data
			if data == nil {
				data = map[string]any{}
			}
			if err := tmpl.Execute(&buf, data); err != nil {
				t.Fatalf("execute template: %v", err)
			}

			if got := buf.String(); got != tc.expected {
				t.Errorf("got %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestTemplateFuncMap_LookPathReturnValue(t *testing.T) {
	t.Parallel()

	fn := templateFuncMap()["lookPath"].(func(string) string)

	got := fn("go")
	want, _ := exec.LookPath("go")
	if got != want {
		t.Errorf("lookPath(\"go\") = %q, want %q", got, want)
	}

	got = fn("crucible_nonexistent_cmd_xyz")
	if got != "" {
		t.Errorf("lookPath(nonexistent) = %q, want empty", got)
	}
}

func TestTemplateFuncMap_EnvReadsSetVar(t *testing.T) {
	t.Setenv("CRUCIBLE_TEST_VAR", "test_value")

	funcs := templateFuncMap()
	tmpl, err := template.New("test").Funcs(funcs).Parse(`{{ env "CRUCIBLE_TEST_VAR" }}`)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, nil); err != nil {
		t.Fatal(err)
	}

	if got := buf.String(); got != "test_value" {
		t.Errorf("got %q, want %q", got, "test_value")
	}
}

func TestRenderTemplate_WithFuncs(t *testing.T) {
	t.Parallel()

	tmplContent := `OS: {{ .os.name }}, Editor: {{ env "EDITOR" | default "vim" }}, Arch: {{ .os.arch | upper }}`
	data := map[string]any{
		"os": map[string]any{
			"name": runtime.GOOS,
			"arch": runtime.GOARCH,
		},
	}

	result, err := renderTemplate("test", tmplContent, data)
	if err != nil {
		t.Fatal(err)
	}

	got := string(result)
	if !bytes.Contains(result, []byte("OS: "+runtime.GOOS)) {
		t.Errorf("expected OS in result, got %q", got)
	}
}
