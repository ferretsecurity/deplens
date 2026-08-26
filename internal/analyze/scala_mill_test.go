package analyze

import (
	"path/filepath"
	"testing"
)

func TestScalaMillFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []DependencyReference
	}{
		{
			name:    "runtime and test dependencies",
			fixture: "mill-runtime-test",
			want: []DependencyReference{
				{PackageType: "maven", Raw: "demo.test::checks:2.0.0", Name: "demo.test:checks", VersionConstraint: "[2.0.0]", VERS: "vers:maven/2.0.0", SourceGroup: "ivyDeps", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeTest},
				{PackageType: "maven", Raw: "demo.web::html:1.2.3", Name: "demo.web:html", VersionConstraint: "[1.2.3]", VERS: "vers:maven/1.2.3", SourceGroup: "ivyDeps", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
			},
		},
		{
			name:    "version constants and plugin dependencies",
			fixture: "mill-version-constants",
			want: []DependencyReference{
				{PackageType: "maven", Raw: "demo:core:${versions.core}", Name: "demo:core", VersionConstraint: "[3.4.5]", VERS: "vers:maven/3.4.5", SourceGroup: "ivyDeps", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "maven", Raw: "demo:runtime:${scalaVersion()}", Name: "demo:runtime", VersionConstraint: "[${scalaVersion()}]", SourceGroup: "ivyDeps", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "maven", Raw: "tools:::compiler-plugin:${versions.plugin}", Name: "tools:compiler-plugin", VersionConstraint: "[0.13.2]", VERS: "vers:maven/0.13.2", SourceGroup: "scalacPluginIvyDeps", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeBuild},
			},
		},
		{
			name:    "build imports and module dependencies",
			fixture: "mill-build-imports",
			want: []DependencyReference{
				{PackageType: "maven", Raw: "build.tools::format-plugin::1.0.0", Name: "build.tools:format-plugin", VersionConstraint: "[1.0.0]", VERS: "vers:maven/1.0.0", SourceGroup: "ivy-imports", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeBuild},
				{PackageType: "maven", Raw: "build.tools::lint-plugin::2.0.0", Name: "build.tools:lint-plugin", VersionConstraint: "[2.0.0]", VERS: "vers:maven/2.0.0", SourceGroup: "ivy-imports", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeBuild},
				{PackageType: "maven", Raw: "demo.runtime::library:4.0.0", Name: "demo.runtime:library", VersionConstraint: "[4.0.0]", VERS: "vers:maven/4.0.0", SourceGroup: "ivyDeps", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "scala", test.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "build.sc")
			if source.Detector != "scala-mill" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
		})
	}
}

func TestScalaMillWithoutDependenciesIsAbsent(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	result, err := Scan(filepath.Join("..", "..", "testdata", "scala", "mill-build"), nil, ruleset)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	source := sourceForPath(t, result, "build.sc")
	if source.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(source.Dependencies) != 0 {
		t.Fatalf("source = %+v", source)
	}
}
