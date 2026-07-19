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
