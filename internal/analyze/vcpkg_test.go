package analyze

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestVcpkgFixturesExtractDependencyReferences(t *testing.T) {
	testCases := []struct {
		name       string
		fixtureDir string
		want       []DependencyReference
	}{
		{
			name:       "string dependencies",
			fixtureDir: "vcpkg-string-dependencies",
			want: []DependencyReference{
				{PackageType: "vcpkg", Raw: "alpha", Name: "alpha", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "vcpkg", Raw: "beta", Name: "beta", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
			},
		},
		{
			name:       "feature dependencies",
			fixtureDir: "vcpkg-feature-dependencies",
			want: []DependencyReference{
				{PackageType: "vcpkg", Raw: "core", Name: "core", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "vcpkg", Raw: "lint-tool", Name: "lint-tool", SourceGroup: "features.tools.dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeOptional},
				{PackageType: "vcpkg", Raw: "test-kit", Name: "test-kit", SourceGroup: "features.tools.dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeOptional},
			},
		},
		{
			name:       "host dependency object",
			fixtureDir: "vcpkg-host-dependencies",
			want: []DependencyReference{
				{PackageType: "vcpkg", Raw: "build-helper", Name: "build-helper", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeBuild},
				{PackageType: "vcpkg", Raw: "runtime-lib", Name: "runtime-lib", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
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
			if source.Detector != "cpp-vcpkg" {
				t.Fatalf("detector = %q, want cpp-vcpkg", source.Detector)
			}
			if want := (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}); source.Analysis != want {
				t.Fatalf("analysis = %+v, want %+v", source.Analysis, want)
			}
			if !reflect.DeepEqual(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, tc.want)
			}
		})
	}
}

func TestVcpkgReturnsConclusiveEmptyForDependencyFreeManifest(t *testing.T) {
	parser, err := newVcpkgParser(vcpkgMatcherConfig{})
	if err != nil {
		t.Fatalf("newVcpkgParser: %v", err)
	}

	result, err := parser.Analyze("vcpkg.json", []byte(`{"name":"sample-port"}`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !result.Recognized {
		t.Fatal("expected recognized vcpkg manifest")
	}
	if result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) {
		t.Fatalf("analysis = %+v", result.Analysis)
	}
	if len(result.Dependencies) != 0 {
		t.Fatalf("dependencies = %+v, want none", result.Dependencies)
	}
}
