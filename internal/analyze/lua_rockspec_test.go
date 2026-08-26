package analyze

import (
	"path/filepath"
	"testing"
)

func TestLuaRocksFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []DependencyReference
	}{
		{
			name:    "runtime dependency list",
			fixture: "runtime-list-1.rockspec",
			want: []DependencyReference{
				luaRocksTestDependency("lua", ">= 5.1"),
				luaRocksTestDependency("networking", ""),
				luaRocksTestDependency("secure", ">= 0.5.1, < 0.8-1"),
			},
		},
		{
			name:    "dynamic identity",
			fixture: "dynamic-identity-1.rockspec",
			want:    []DependencyReference{luaRocksTestDependency("lua", ">= 5.0, < 5.6")},
		},
		{
			name:    "concatenated version",
			fixture: "concatenated-version-1.rockspec",
			want:    []DependencyReference{luaRocksTestDependency("lua", ">= 5.1, < 5.6")},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "lua", "rockspec"), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, test.fixture)
			if source.Detector != "lua-rockspec" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
		})
	}
}

func TestLuaRocksManifestWithoutDependenciesIsCompleteAndAbsent(t *testing.T) {
	parser, err := newLuaRocksParser(luaRocksMatcherConfig{})
	if err != nil {
		t.Fatalf("newLuaRocksParser: %v", err)
	}
	result, err := parser.Analyze("empty.rockspec", []byte("package = \"empty\"\nversion = \"1.0-1\"\n"))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(result.Dependencies) != 0 {
		t.Fatalf("result = %+v", result)
	}
}

func luaRocksTestDependency(name, constraint string) DependencyReference {
	dependency := DependencyReference{
		PackageType:  "luarocks",
		Raw:          name,
		Name:         name,
		SourceGroup:  "dependencies",
		OriginKind:   OriginRegistry,
		Relationship: RelationshipDirect,
		Scope:        ScopeRuntime,
	}
	if constraint != "" {
		dependency.Raw += "@" + constraint
		dependency.VersionConstraint = constraint
	}
	return dependency
}
