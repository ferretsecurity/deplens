package analyze

import (
	"path/filepath"
	"testing"
)

func TestRakuMetaFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []DependencyReference
	}{
		{
			name:    "runtime and test dependencies with qualifiers",
			fixture: "runtime-and-test",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "DateTime::Parse", Name: "DateTime::Parse", SourceGroup: "depends", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "generic", Raw: "IO::Socket::SSL:ver:<0.0.4+>:auth<raku-community-modules>", Name: "IO::Socket::SSL", VersionConstraint: "0.0.4+", SourceGroup: "depends", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"auth": "raku-community-modules"}},
				{PackageType: "generic", Raw: "JSON::Fast", Name: "JSON::Fast", SourceGroup: "test-depends", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeTest},
			},
		},
		{
			name:    "runtime dependency",
			fixture: "runtime-only",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "HTML::Escape", Name: "HTML::Escape", SourceGroup: "depends", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
			},
		},
		{
			name:    "build dependency",
			fixture: "build-only",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "LibraryMake", Name: "LibraryMake", SourceGroup: "build-depends", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeBuild},
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "raku", test.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "META6.json")
			if source.Detector != "raku-meta" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
		})
	}
}

func TestRakuMetaWithoutDependenciesIsCompleteAndAbsent(t *testing.T) {
	parser, err := newRakuMetaParser(rakuMetaMatcherConfig{})
	if err != nil {
		t.Fatalf("newRakuMetaParser: %v", err)
	}
	result, err := parser.Analyze("META6.json", []byte(`{"name":"Example"}`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !result.Recognized || result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(result.Dependencies) != 0 {
		t.Fatalf("result = %+v", result)
	}
}
