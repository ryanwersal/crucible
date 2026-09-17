package modules

import "testing"

func TestScriptSudoValidation(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		option  string
		want    bool
		invalid bool
	}{
		{name: "default"},
		{name: "enabled", option: ", sudo: true", want: true},
		{name: "disabled", option: ", sudo: false"},
		{name: "string", option: `, sudo: "false"`, invalid: true},
		{name: "number", option: ", sudo: 1", invalid: true},
		{name: "null", option: ", sudo: null", invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			vm, declarations := setupModule(t)
			_, err := vm.RunString(`c.script("test", {check: "false", install: "true"` + tc.option + `})`)
			if (err != nil) != tc.invalid {
				t.Fatalf("err = %v, want invalid = %v", err, tc.invalid)
			}
			if tc.invalid {
				if len(*declarations) != 0 {
					t.Fatal("invalid option created a declaration")
				}
				return
			}
			if len(*declarations) != 1 || (*declarations)[0].ScriptSudo != tc.want {
				t.Fatalf("unexpected declarations: %+v", *declarations)
			}
		})
	}
}
