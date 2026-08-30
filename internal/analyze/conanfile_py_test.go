package analyze

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestConanfilePyFixturesExtractDependencyReferences(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	testCases := []struct {
		name       string
		fixtureDir string
		want       []DependencyReference
	}{
		{
			name:       "toolchain recipe has no dependencies",
			fixtureDir: "conanfile-py-toolchain",
			want:       []DependencyReference{},
		},
		{
			name:       "requirements and build requirements",
			fixtureDir: "conanfile-py-requirements",
			want: []DependencyReference{
				{PackageType: "conan", Raw: "antlr4-cppruntime/4.13.1", Name: "antlr4-cppruntime", VersionConstraint: "4.13.1", SourceGroup: "requires", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "conan", Raw: "fmt/11.0.2", Name: "fmt", VersionConstraint: "11.0.2", SourceGroup: "requires", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "conan", Raw: "range-v3/0.12.0", Name: "range-v3", VersionConstraint: "0.12.0", SourceGroup: "requires", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "conan", Raw: "tree-gen/1.0.9", Name: "tree-gen", VersionConstraint: "1.0.9", SourceGroup: "requires", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "conan", Raw: "gtest/1.15.0", Name: "gtest", VersionConstraint: "1.15.0", SourceGroup: "test_requires", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeTest},
				{PackageType: "conan", Raw: "emsdk/3.1.50", Name: "emsdk", VersionConstraint: "3.1.50", SourceGroup: "tool_requires", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeBuild},
				{PackageType: "conan", Raw: "tree-gen/1.0.9", Name: "tree-gen", VersionConstraint: "1.0.9", SourceGroup: "tool_requires", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeBuild},
				{PackageType: "conan", Raw: "zulu-openjdk/21.0.1", Name: "zulu-openjdk", VersionConstraint: "21.0.1", SourceGroup: "tool_requires", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeBuild},
			},
		},
		{
			name:       "library recipe has no dependencies",
			fixtureDir: "conanfile-py-library",
			want:       []DependencyReference{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "cpp", tc.fixtureDir), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("sources = %+v, want one", result.Sources)
			}
			source := result.Sources[0]
			if source.Detector != "cpp-conanfile-py" {
				t.Fatalf("detector = %q, want cpp-conanfile-py", source.Detector)
			}
			wantAnalysis := SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}
			if len(tc.want) > 0 {
				wantAnalysis.Presence = PresencePresent
			}
			if source.Analysis != wantAnalysis {
				t.Fatalf("analysis = %+v, want %+v", source.Analysis, wantAnalysis)
			}
			if !reflect.DeepEqual(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, tc.want)
			}
		})
	}
}
