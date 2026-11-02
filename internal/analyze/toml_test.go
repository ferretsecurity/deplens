package analyze

import "testing"

func TestExtractTOMLDependenciesSetsStructuredFieldsForStrings(t *testing.T) {
	nodes := []tomlMatchedValue{
		{value: "requests>=2.28.0", sourceGroup: ""},
		{value: "flask", sourceGroup: ""},
	}
	got := extractTOMLDependencies(nodes, tomlQuery{})
	if len(got) != 2 {
		t.Fatalf("expected 2 deps, got %d", len(got))
	}
	if got[0].Raw != "requests>=2.28.0" || got[0].Name != "requests" || got[0].VersionConstraint != ">=2.28.0" {
		t.Errorf("first dep: %+v", got[0])
	}
	if got[1].Raw != "flask" || got[1].Name != "flask" || got[1].VersionConstraint != "" {
		t.Errorf("second dep: %+v", got[1])
	}
}

func TestExtractTOMLDependenciesSeparatesDirectReferences(t *testing.T) {
	raw := "demo @ git+https://example.test/demo.git@v1#subdirectory=python"
	got := extractTOMLDependencies([]tomlMatchedValue{{value: raw}}, tomlQuery{})
	if len(got) != 1 || got[0].Raw != raw || got[0].Name != "demo" || got[0].VersionConstraint != "" || got[0].OriginKind != OriginGit || got[0].Attributes["source_url"] != "git+https://example.test/demo.git#subdirectory=python" || got[0].Attributes["source_ref"] != "v1" {
		t.Fatalf("direct reference = %+v", got)
	}
}
