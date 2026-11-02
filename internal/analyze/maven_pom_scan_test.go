package analyze

import (
	"path/filepath"
	"testing"
)

func TestScanMavenPOMProfileOnlyDependencies(t *testing.T) {
	result, err := Scan(filepath.Join("..", "..", "testdata", "java", "maven-pom-profiles"), nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	source := sourceForPath(t, result, "pom.xml")
	if source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) || len(source.Dependencies) != 2 {
		t.Fatalf("source = %+v", source)
	}
	for _, dependency := range source.Dependencies {
		if dependency.PackageType != "maven" || dependency.SourceGroup != "profiles.benchmark.dependencies" || dependency.Relationship != RelationshipDirect || dependency.VERS != "vers:maven/1.37" {
			t.Fatalf("dependency = %+v", dependency)
		}
		if dependency.Attributes["profile"] != "benchmark" || dependency.Attributes["conditional"] != "true" || dependency.Attributes["maven_activation"] != "<activeByDefault>true</activeByDefault>" {
			t.Fatalf("profile attributes = %+v", dependency.Attributes)
		}
	}
	if source.Dependencies[0].Name != "org.openjdk.jmh:jmh-core" || source.Dependencies[1].Name != "org.openjdk.jmh:jmh-generator-annprocess" {
		t.Fatalf("dependencies = %+v", source.Dependencies)
	}
}

func TestScanMavenPOMProfileManagementOnly(t *testing.T) {
	result, err := Scan(filepath.Join("..", "..", "testdata", "java", "maven-pom-profile-management"), nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	source := sourceForPath(t, result, "pom.xml")
	if source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) || len(source.Dependencies) != 1 {
		t.Fatalf("source = %+v", source)
	}
	dependency := source.Dependencies[0]
	assertDependency(t, dependency, "org.example:platform", "[2.0]", "profiles.production.dependencyManagement", RelationshipInconclusive, ScopeBuild)
	if dependency.PackageType != "maven" || dependency.Attributes["profile"] != "production" || dependency.Attributes["conditional"] != "true" || dependency.Attributes["type"] != "pom" {
		t.Fatalf("dependency = %+v", dependency)
	}
}
