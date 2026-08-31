package analyze

import (
	"path/filepath"
	"testing"
)

func TestClojureBootFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []DependencyReference
	}{
		{
			name:    "client dependencies and test scope",
			fixture: "boot-client",
			want: []DependencyReference{
				{PackageType: "maven", Raw: "demo/client@1.2.3", Name: "demo/client", VersionConstraint: "[1.2.3]", VERS: "vers:maven/1.2.3", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "maven", Raw: "demo/client-test@1.2.4", Name: "demo/client-test", VersionConstraint: "[1.2.4]", VERS: "vers:maven/1.2.4", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeTest},
			},
		},
		{
			name:    "service dependencies after version",
			fixture: "boot-service",
			want: []DependencyReference{
				{PackageType: "maven", Raw: "demo/jdbc@3.4.5", Name: "demo/jdbc", VersionConstraint: "[3.4.5]", VERS: "vers:maven/3.4.5", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "maven", Raw: "demo/json@2.3.4", Name: "demo/json", VersionConstraint: "[2.3.4]", VERS: "vers:maven/2.3.4", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
			},
		},
		{
			name:    "library dependencies with comments",
			fixture: "boot-library",
			want: []DependencyReference{
				{PackageType: "maven", Raw: "demo/library@4.5.6", Name: "demo/library", VersionConstraint: "[4.5.6]", VERS: "vers:maven/4.5.6", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "maven", Raw: "demo/test-runner@4.5.7-SNAPSHOT", Name: "demo/test-runner", VersionConstraint: "[4.5.7-SNAPSHOT]", VERS: "vers:maven/4.5.7-SNAPSHOT", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeTest},
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "clojure", test.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "build.boot")
			if source.Detector != "clojure-boot" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
		})
	}
}

func TestClojureBootStaticEnvironmentWithoutDependenciesIsAbsent(t *testing.T) {
	parser, err := newClojureBootParser(clojureBootMatcherConfig{})
	if err != nil {
		t.Fatalf("newClojureBootParser: %v", err)
	}
	result, err := parser.Analyze("build.boot", []byte(`(set-env! :source-paths #{"src"})`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(result.Dependencies) != 0 {
		t.Fatalf("result = %+v", result)
	}
}
