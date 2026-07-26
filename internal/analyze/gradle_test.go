package analyze

import (
	"slices"
	"strings"
	"testing"
)

func TestGradleBuildParserExtractsGroovyKotlinAndMapNotation(t *testing.T) {
	parser, _ := newGradleBuildParser(gradleBuildMatcherConfig{})
	result, err := parser.Analyze("build.gradle", []byte(`
dependencies {
  implementation("org.example:core:1.2.3")
  testImplementation 'org.example:test-kit:2.0'
  runtimeOnly group: "org.example", name: "runtime", version: "3.1"
  // implementation("ignored:comment:1.0")
}
`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
		t.Fatalf("unexpected analysis: %+v", result.Analysis)
	}
	if got, want := dependencyNames(result.Dependencies), []string{
		"org.example:core:1.2.3",
		"org.example:runtime:3.1",
		"org.example:test-kit:2.0",
	}; !slices.Equal(got, want) {
		t.Fatalf("dependencies: got %v want %v", got, want)
	}
	if result.Dependencies[0].VersionConstraint != "[1.2.3]" || result.Dependencies[0].Scope != ScopeRuntime {
		t.Fatalf("unexpected structured dependency: %+v", result.Dependencies[0])
	}
}

func TestGradleBuildParserReportsStaticAndDynamicAsPartial(t *testing.T) {
	parser, _ := newGradleBuildParser(gradleBuildMatcherConfig{})
	result, err := parser.Analyze("build.gradle.kts", []byte(`
dependencies {
  implementation("org.example:core:1.2.3")
  implementation(libs.jackson.databind)
  implementation("org.example:dynamic:$libraryVersion")
}
`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Analysis.Extraction != ExtractionPartial || len(result.Dependencies) != 1 || len(result.Diagnostics) != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestGradleBuildParserExtractsMultilineMapNotationAndReportsDynamicVersion(t *testing.T) {
	parser, _ := newGradleBuildParser(gradleBuildMatcherConfig{})
	result, err := parser.Analyze("build.gradle.kts", []byte(`
dependencies {
  implementation(
    group = "org.example",
    name = "core",
    version = "1.2.3",
  )
  runtimeOnly(group = "org.example", name = "dynamic", version = libraryVersion)
}
`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Analysis.Extraction != ExtractionPartial || len(result.Dependencies) != 2 || len(result.Diagnostics) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestGradleBuildParserReportsDynamicOnlyAsUnsupported(t *testing.T) {
	parser, _ := newGradleBuildParser(gradleBuildMatcherConfig{})
	result, err := parser.Analyze("build.gradle", []byte(`dependencies { implementation(libs.core) }`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionUnsupported}) {
		t.Fatalf("unexpected analysis: %+v", result.Analysis)
	}
}

func TestGradleBuildParserRejectsUnterminatedStrings(t *testing.T) {
	parser, _ := newGradleBuildParser(gradleBuildMatcherConfig{})
	_, err := parser.Analyze("build.gradle", []byte(`implementation("org.example:broken:1.0)`))
	if err == nil || !strings.Contains(err.Error(), "unterminated") {
		t.Fatalf("expected unterminated string error, got %v", err)
	}
}

func TestGradleLockParserExtractsResolvedModules(t *testing.T) {
	parser, _ := newGradleLockParser(gradleLockMatcherConfig{})
	result, err := parser.Analyze("gradle.lockfile", []byte(`
# generated
org.slf4j:slf4j-api:2.0.13=compileClasspath,runtimeClasspath
com.fasterxml.jackson.core:jackson-core:2.17.1=runtimeClasspath
empty=
`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(result.Dependencies) != 2 || result.Dependencies[0].Relationship != RelationshipInconclusive {
		t.Fatalf("unexpected dependencies: %+v", result.Dependencies)
	}
	if result.Dependencies[0].Attributes["configurations"] == "" {
		t.Fatalf("expected configurations attribute: %+v", result.Dependencies[0])
	}
}

func TestGradleLockParserReturnsAbsentForEmptyMarker(t *testing.T) {
	parser, _ := newGradleLockParser(gradleLockMatcherConfig{})
	result, err := parser.Analyze("gradle.lockfile", []byte("empty=\n"))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) {
		t.Fatalf("unexpected analysis: %+v", result.Analysis)
	}
}

func TestGradleVersionCatalogParserResolvesReferencesAndInlineEntries(t *testing.T) {
	parser, _ := newGradleVersionCatalogParser(gradleVersionCatalogMatcherConfig{})
	result, err := parser.Analyze("libs.versions.toml", []byte(`
[versions]
jackson = "2.17.1"

[libraries]
jackson = { module = "com.fasterxml.jackson.core:jackson-databind", version.ref = "jackson" }
guava = "com.google.guava:guava:33.2.0-jre"
`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(result.Dependencies) != 2 {
		t.Fatalf("unexpected dependencies: %+v", result.Dependencies)
	}
	for _, dependency := range result.Dependencies {
		if dependency.Attributes["alias"] == "jackson" && dependency.Attributes["version_ref"] != "jackson" {
			t.Fatalf("expected resolved version ref: %+v", dependency)
		}
	}
}

func TestGradleVersionCatalogParserReportsUnknownVersionRef(t *testing.T) {
	parser, _ := newGradleVersionCatalogParser(gradleVersionCatalogMatcherConfig{})
	result, err := parser.Analyze("libs.versions.toml", []byte(`
[libraries]
broken = { module = "org.example:broken", version.ref = "missing" }
`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Analysis.Extraction != ExtractionUnsupported || len(result.Diagnostics) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}
