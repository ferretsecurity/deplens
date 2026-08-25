package analyze

import (
	"path/filepath"
	"testing"
)

func TestClojureProjectCLJFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []DependencyReference
	}{
		{
			name:    "root dependencies",
			fixture: "project-clj-root-dependencies",
			want: []DependencyReference{
				{PackageType: "maven", Raw: "example/core@1.2.3", Name: "example/core", VersionConstraint: "[1.2.3]", VERS: "vers:maven/1.2.3", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "maven", Raw: "example/web@2.3.4", Name: "example/web", VersionConstraint: "[2.3.4]", VERS: "vers:maven/2.3.4", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
			},
		},
		{
			name:    "profile dependencies and root plugin",
			fixture: "project-clj-profile-dependencies",
			want: []DependencyReference{
				{PackageType: "maven", Raw: "example/release@3.0.0", Name: "example/release", VersionConstraint: "[3.0.0]", VERS: "vers:maven/3.0.0", SourceGroup: "plugins", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeBuild},
				{PackageType: "maven", Raw: "example/repl@2.0.0", Name: "example/repl", VersionConstraint: "[2.0.0]", VERS: "vers:maven/2.0.0", SourceGroup: "profiles.dev.dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeDevelopment},
				{PackageType: "maven", Raw: "example/api@1.0.0", Name: "example/api", VersionConstraint: "[1.0.0]", VERS: "vers:maven/1.0.0", SourceGroup: "profiles.provided.dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeBuild},
				{PackageType: "maven", Raw: "example/worker@3.0.0", Name: "example/worker", VersionConstraint: "[3.0.0]", VERS: "vers:maven/3.0.0", SourceGroup: "profiles.repl.dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
			},
		},
		{
			name:    "profile plugins and root dependencies",
			fixture: "project-clj-profile-plugins",
			want: []DependencyReference{
				{PackageType: "maven", Raw: "example/base@6.0.0", Name: "example/base", VersionConstraint: "[6.0.0]", VERS: "vers:maven/6.0.0", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "maven", Raw: "example/chart@5.0.0", Name: "example/chart", VersionConstraint: "[5.0.0]", VERS: "vers:maven/5.0.0", SourceGroup: "profiles.dev.dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeDevelopment},
				{PackageType: "maven", Raw: "example/linter@4.0.0", Name: "example/linter", VersionConstraint: "[4.0.0]", VERS: "vers:maven/4.0.0", SourceGroup: "profiles.dev.plugins", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeBuild},
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
			source := sourceForPath(t, result, "project.clj")
			if source.Detector != "clojure-project-clj" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
		})
	}
}

func TestClojureProjectCLJWithoutDependenciesIsAbsent(t *testing.T) {
	parser, err := newClojureProjectCLJParser(clojureProjectCLJMatcherConfig{})
	if err != nil {
		t.Fatalf("newClojureProjectCLJParser: %v", err)
	}
	result, err := parser.Analyze("project.clj", []byte(`(defproject example/empty "0.1.0" :description "no dependencies")`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(result.Dependencies) != 0 {
		t.Fatalf("result = %+v", result)
	}
}
