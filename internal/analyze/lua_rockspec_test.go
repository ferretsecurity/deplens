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
			name:    "build dependency list",
			fixture: "build-dependencies-3.0-1.rockspec",
			want: []DependencyReference{
				luaRocksTestDependencyInGroup("luarocks-build-hooks", ">= 0.8.0", "build_dependencies", ScopeBuild),
				luaRocksTestDependencyInGroup("luarocks-build-rust-mlua", ">= 0.2.6", "build_dependencies", ScopeBuild),
			},
		},
		{
			name:    "test dependency list",
			fixture: "test-dependencies-3.0-1.rockspec",
			want: []DependencyReference{
				luaRocksTestDependencyInGroup("busted", ">= 2.2", "test_dependencies", ScopeTest),
				luaRocksTestDependencyInGroup("luacov", ">= 0.15.0", "test_dependencies", ScopeTest),
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
	return luaRocksTestDependencyInGroup(name, constraint, "dependencies", ScopeRuntime)
}

func luaRocksTestDependencyInGroup(name, constraint, sourceGroup string, scope DependencyScope) DependencyReference {
	dependency := DependencyReference{
		PackageType:  "luarocks",
		Raw:          name,
		Name:         name,
		SourceGroup:  sourceGroup,
		OriginKind:   OriginRegistry,
		Relationship: RelationshipDirect,
		Scope:        scope,
	}
	if constraint != "" {
		dependency.Raw += "@" + constraint
		dependency.VersionConstraint = constraint
	}
	return dependency
}
