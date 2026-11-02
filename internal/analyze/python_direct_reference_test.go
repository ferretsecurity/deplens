package analyze

import "testing"

func TestPythonGitFileReferences(t *testing.T) {
	const named = "demo @ git+file:///tmp/demo.git@v1"
	requirements, _ := newPyRequirementsMatcher(pyRequirementsMatcherConfig{})
	conda, _ := newCondaEnvironmentParser(struct{}{})
	r, err := requirements.Analyze("requirements.txt", []byte(named))
	if err != nil {
		t.Fatal(err)
	}
	e, err := requirements.Analyze("requirements.txt", []byte("-e git+file:///tmp/demo.git@v1#egg=demo"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := conda.Analyze("environment.yml", []byte("dependencies:\n  - pip:\n    - "+named+"\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		deps []DependencyReference
	}{
		{"requirements", r.Dependencies},
		{"editable", e.Dependencies},
		{"toml", extractTOMLDependencies([]tomlMatchedValue{{value: named}}, tomlQuery{})},
		{"conda", c.Dependencies},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.deps) != 1 {
				t.Fatalf("dependencies = %+v", tc.deps)
			}
			d := tc.deps[0]
			if d.Name != "demo" || d.OriginKind != OriginGit || d.VersionConstraint != "" || d.Attributes["source_url"] != "git+file:///tmp/demo.git" || d.Attributes["source_ref"] != "v1" {
				t.Fatalf("Git file reference = %+v", d)
			}
		})
	}
}
