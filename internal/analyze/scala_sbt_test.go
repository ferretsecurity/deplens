package analyze

import (
	"path/filepath"
	"testing"
)

func TestScalaSBTBuildFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []DependencyReference
	}{
		{
			name:    "cross version dependency",
			fixture: "sbt-cross-version",
			want: []DependencyReference{
				{PackageType: "maven", Raw: "sample.xml%%codec@2.1.0", Name: "sample.xml:codec", VersionConstraint: "[2.1.0]", VERS: "vers:maven/2.1.0", SourceGroup: "libraryDependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
			},
		},
		{
			name:    "runtime and test dependencies",
			fixture: "sbt-runtime-and-test",
			want: []DependencyReference{
				{PackageType: "maven", Raw: "sample.logging%logger@1.4.14", Name: "sample.logging:logger", VersionConstraint: "[1.4.14]", VERS: "vers:maven/1.4.14", SourceGroup: "libraryDependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "maven", Raw: "sample.runtime%%engine@1.1.0", Name: "sample.runtime:engine", VersionConstraint: "[1.1.0]", VERS: "vers:maven/1.1.0", SourceGroup: "libraryDependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "maven", Raw: "sample.test%%checks@3.2.19", Name: "sample.test:checks", VersionConstraint: "[3.2.19]", VERS: "vers:maven/3.2.19", SourceGroup: "libraryDependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeTest},
			},
		},
		{
			name:    "Scala.js dependencies",
			fixture: "sbt-scalajs",
			want: []DependencyReference{
				{PackageType: "maven", Raw: "sample.js%%%dom@0.8.2", Name: "sample.js:dom", VersionConstraint: "[0.8.2]", VERS: "vers:maven/0.8.2", SourceGroup: "libraryDependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "maven", Raw: "sample.js%%%widgets@0.8.1", Name: "sample.js:widgets", VersionConstraint: "[0.8.1]", VERS: "vers:maven/0.8.1", SourceGroup: "libraryDependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
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
			source := sourceForPath(t, result, "build.sbt")
			if source.Detector != "scala-sbt-build" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
		})
	}
}

func TestScalaSBTBuildWithoutDependenciesIsAbsent(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	result, err := Scan(filepath.Join("..", "..", "testdata", "scala", "sbt-build"), nil, ruleset)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	source := sourceForPath(t, result, "build.sbt")
	if source.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(source.Dependencies) != 0 {
		t.Fatalf("source = %+v", source)
	}
}
