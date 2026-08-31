package analyze

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestConanfileFixturesExtractDependencyReferences(t *testing.T) {
	testCases := []struct {
		name       string
		fixtureDir string
		want       []DependencyReference
	}{
		{
			name:       "runtime and test requirements",
			fixtureDir: "conanfile-runtime-test",
			want: []DependencyReference{
				{PackageType: "conan", Raw: "boost/1.85.0", Name: "boost", VersionConstraint: "1.85.0", SourceGroup: "requires", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "conan", Raw: "eigen/3.4.0", Name: "eigen", VersionConstraint: "3.4.0", SourceGroup: "requires", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "conan", Raw: "catch2/3.6.0", Name: "catch2", VersionConstraint: "3.6.0", SourceGroup: "test_requires", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeTest},
				{PackageType: "conan", Raw: "gsl/2.7.1", Name: "gsl", VersionConstraint: "2.7.1", SourceGroup: "test_requires", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeTest},
			},
		},
		{
			name:       "user channel and build requirements",
			fixtureDir: "conanfile-user-channel",
			want: []DependencyReference{
				{PackageType: "conan", Raw: "Catch2/2.5.0@catchorg/stable", Name: "Catch2", VersionConstraint: "2.5.0", SourceGroup: "build_requires", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeBuild, Attributes: map[string]string{"user": "catchorg", "channel": "stable"}},
				{PackageType: "conan", Raw: "range-v3/0.4.0@ericniebler/stable", Name: "range-v3", VersionConstraint: "0.4.0", SourceGroup: "requires", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"user": "ericniebler", "channel": "stable"}},
			},
		},
		{
			name:       "version range",
			fixtureDir: "conanfile-version-range",
			want: []DependencyReference{
				{PackageType: "conan", Raw: "openimageio/[>=2.4 <4]", Name: "openimageio", VersionConstraint: "[>=2.4 <4]", SourceGroup: "requires", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
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
			if source.Detector != "cpp-conanfile" {
				t.Fatalf("detector = %q, want cpp-conanfile", source.Detector)
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

func TestConanfileReturnsConclusiveEmptyForDependencyFreeManifest(t *testing.T) {
	parser, err := newConanfileParser(conanfileMatcherConfig{})
	if err != nil {
		t.Fatalf("newConanfileParser: %v", err)
	}

	result, err := parser.Analyze("conanfile.txt", []byte("[generators]\nCMakeDeps\n"))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !result.Recognized {
		t.Fatal("expected recognized Conan manifest")
	}
	if result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) {
		t.Fatalf("analysis = %+v", result.Analysis)
	}
	if len(result.Dependencies) != 0 {
		t.Fatalf("dependencies = %+v, want none", result.Dependencies)
	}
}
