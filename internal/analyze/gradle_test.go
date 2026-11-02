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

func TestGradleBuildParserKeepsConstraintsSeparateFromDirectDependencies(t *testing.T) {
	tests := []struct {
		name    string
		content string
		direct  bool
	}{
		{
			name: "multiline constraint only",
			content: `dependencies {
  constraints {
    implementation("org.example:core:1.2")
  }
}`,
		},
		{
			name:    "inline constraint only",
			content: `dependencies { constraints { implementation("org.example:core:1.2") } }`,
		},
		{
			name: "same coordinate declared directly",
			content: `dependencies {
  implementation("org.example:core:1.2")
  constraints {
    implementation("org.example:core:1.2")
  }
}`,
			direct: true,
		},
		{
			name:    "inline direct declaration after constraint",
			content: `dependencies { constraints { implementation("org.example:core:1.2") }; implementation("org.example:core:1.2") }`,
			direct:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser, _ := newGradleBuildParser(gradleBuildMatcherConfig{})
			result, err := parser.Analyze("build.gradle.kts", []byte(tt.content))
			if err != nil {
				t.Fatalf("Analyze: %v", err)
			}
			if result.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("unexpected analysis: %+v", result.Analysis)
			}
			wantCount := 1
			if tt.direct {
				wantCount++
			}
			if len(result.Dependencies) != wantCount {
				t.Fatalf("expected %d dependencies, got %+v", wantCount, result.Dependencies)
			}
			groups := make(map[string]Relationship)
			for _, dependency := range result.Dependencies {
				if dependency.Raw != "org.example:core:1.2" || dependency.Name != "org.example:core" ||
					dependency.VersionConstraint != "[1.2]" || dependency.Scope != ScopeRuntime {
					t.Fatalf("unexpected dependency: %+v", dependency)
				}
				groups[dependency.SourceGroup] = dependency.Relationship
			}
			if groups["constraints.implementation"] != RelationshipInconclusive {
				t.Fatalf("expected inconclusive constraint, got %+v", result.Dependencies)
			}
			if tt.direct && groups["implementation"] != RelationshipDirect {
				t.Fatalf("expected separate direct declaration, got %+v", result.Dependencies)
			}
		})
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

func TestGradleBuildParserPreservesConstraintContextAcrossNotationAndClosures(t *testing.T) {
	parser, _ := newGradleBuildParser(gradleBuildMatcherConfig{})
	result, err := parser.Analyze("build.gradle.kts", []byte(`
val message = "dependencies { constraints {"
dependencies {
  implementation("org.example:core:1.2") {
    because("constraints { with an escaped quote \" and brace }")
  }
  constraints {
    api 'org.example:api:2.0'
    testImplementation(platform("org.example:test-platform:3.0")) {
      because('a closing brace } and constraints { are text')
    }
    compileOnly(
      group = "org.example",
      name = "compile",
      version = "4.0",
    )
    nested { runtimeOnly enforcedPlatform('org.example:runtime-platform:5.0') }
  }
  notconstraints { implementation("org.example:unconstrained:6.0") }
  runtimeOnly("org.example:after:7.0")
}
dependencies { implementation("org.example:other:8.0") }
`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
		t.Fatalf("unexpected analysis: %+v", result.Analysis)
	}
	want := map[string]struct {
		group        string
		relationship Relationship
		scope        DependencyScope
	}{
		"org.example:core:1.2":             {"implementation", RelationshipDirect, ScopeRuntime},
		"org.example:api:2.0":              {"constraints.api", RelationshipInconclusive, ScopeRuntime},
		"org.example:test-platform:3.0":    {"constraints.testImplementation", RelationshipInconclusive, ScopeTest},
		"org.example:compile:4.0":          {"constraints.compileOnly", RelationshipInconclusive, ScopeBuild},
		"org.example:runtime-platform:5.0": {"constraints.runtimeOnly", RelationshipInconclusive, ScopeRuntime},
		"org.example:unconstrained:6.0":    {"implementation", RelationshipDirect, ScopeRuntime},
		"org.example:after:7.0":            {"runtimeOnly", RelationshipDirect, ScopeRuntime},
		"org.example:other:8.0":            {"implementation", RelationshipDirect, ScopeRuntime},
	}
	if len(result.Dependencies) != len(want) {
		t.Fatalf("expected %d dependencies, got %+v", len(want), result.Dependencies)
	}
	for _, dependency := range result.Dependencies {
		expected, ok := want[dependency.Raw]
		if !ok || dependency.SourceGroup != expected.group || dependency.Relationship != expected.relationship || dependency.Scope != expected.scope {
			t.Fatalf("unexpected dependency: %+v", dependency)
		}
	}
}

func TestGradleBuildParserKeepsConstraintDiagnostics(t *testing.T) {
	parser, _ := newGradleBuildParser(gradleBuildMatcherConfig{})
	result, err := parser.Analyze("build.gradle.kts", []byte(`
dependencies {
  constraints {
    implementation(libs.core)
    implementation("org.example:dynamic:$libraryVersion")
    runtimeOnly(group = "org.example", name = "runtime", version = libraryVersion)
    testImplementation(group = "org.example", name = artifactName, version = "2.0")
  }
  implementation(libs.core)
}
`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionPartial}) || len(result.Dependencies) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	dependency := result.Dependencies[0]
	if dependency.Name != "org.example:runtime" || dependency.VersionConstraint != "" ||
		dependency.SourceGroup != "constraints.runtimeOnly" || dependency.Relationship != RelationshipInconclusive || dependency.Scope != ScopeRuntime {
		t.Fatalf("unexpected dependency: %+v", dependency)
	}
	wantMessages := []string{
		"dynamic Gradle dependency in configuration implementation could not be extracted: org.example:dynamic:$libraryVersion",
		"dynamic Gradle version in configuration runtimeOnly could not be extracted",
		"dynamic Gradle map dependency in configuration testImplementation could not be fully extracted",
		"version-catalog alias libs.core could not be resolved from this build file",
	}
	if len(result.Diagnostics) != len(wantMessages) {
		t.Fatalf("unexpected diagnostics: %+v", result.Diagnostics)
	}
	for _, message := range wantMessages {
		if !slices.ContainsFunc(result.Diagnostics, func(d Diagnostic) bool { return d.Message == message }) {
			t.Errorf("missing diagnostic %q in %+v", message, result.Diagnostics)
		}
	}
}

func TestGradleBuildParserRejectsUnterminatedConstraintBlock(t *testing.T) {
	parser, _ := newGradleBuildParser(gradleBuildMatcherConfig{})
	_, err := parser.Analyze("build.gradle.kts", []byte(`dependencies { constraints { implementation("org.example:core:1.2") }`))
	if err == nil || !strings.Contains(err.Error(), "unterminated dependencies block") {
		t.Fatalf("expected unterminated block error, got %v", err)
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

func TestGradleBuildParserOnlyReadsDependencyBlocksAndKeepsPlatformConfiguration(t *testing.T) {
	parser, _ := newGradleBuildParser(gradleBuildMatcherConfig{})
	result, err := parser.Analyze("build.gradle.kts", []byte(`
tasks.register("log") {
  doLast { logger.lifecycle("org.example:not-a-dependency:1.0") }
}
dependencies {
  implementation(platform("org.example:platform:1.2.3"))
}
`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if got, want := dependencyNames(result.Dependencies), []string{"org.example:platform:1.2.3"}; !slices.Equal(got, want) {
		t.Fatalf("dependencies: got %v want %v", got, want)
	}
	if dependency := result.Dependencies[0]; dependency.SourceGroup != "implementation" || dependency.Scope != ScopeRuntime {
		t.Fatalf("unexpected dependency: %+v", dependency)
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

func TestGradleBuildParserDoesNotExtractInlineReasons(t *testing.T) {
	for _, path := range []string{"build.gradle", "build.gradle.kts"} {
		t.Run(path, func(t *testing.T) {
			parser, _ := newGradleBuildParser(gradleBuildMatcherConfig{})
			result, err := parser.Analyze(path, []byte(`dependencies {
  implementation("org.example:core:1.2") { because("align with org.example:other:2.0") }
  constraints {
    implementation("org.example:core:1.2") { because("align with org.example:other:2.0") }
  }
}`))
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Dependencies) != 2 {
				t.Fatalf("expected only direct and constraint declarations, got %+v", result.Dependencies)
			}
			for _, dependency := range result.Dependencies {
				if dependency.Name != "org.example:core" {
					t.Fatalf("unexpected dependency from reason: %+v", dependency)
				}
			}
		})
	}
}
