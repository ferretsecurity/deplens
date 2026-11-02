package analyze

import (
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
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

func TestGradleVersionCatalogParserPreservesRichVersions(t *testing.T) {
	tests := []struct {
		name       string
		version    string
		attributes map[string]string
	}{
		{"prefer", `{ prefer = "1.5" }`, map[string]string{"version_prefer": "1.5"}},
		{"require prefer reject", `{ require = "[1.0,2.0)", prefer = "1.5", reject = ["1.4", "1.6"] }`, map[string]string{"version_require": "[1.0,2.0)", "version_prefer": "1.5", "version_reject": `["1.4","1.6"]`}},
		{"strictly", `{ strictly = "[1.0,2.0)" }`, map[string]string{"version_strictly": "[1.0,2.0)"}},
		{"reject only", `{ reject = ["1.4"] }`, map[string]string{"version_reject": `["1.4"]`}},
		{"reject all true", `{ rejectAll = true }`, map[string]string{"version_reject_all": "true"}},
		{"reject all false", `{ rejectAll = false }`, map[string]string{"version_reject_all": "false"}},
		{"empty reject", `{ reject = [] }`, map[string]string{"version_reject": `[]`}},
		{"unknown fields", `{ prefer = "1.5", "future.rule" = { "quoted key" = "keep\tthis" } }`, map[string]string{"version_prefer": "1.5"}},
		{"malformed prefer", `{ prefer = 15 }`, nil},
		{"malformed reject", `{ reject = ["1.4", 16] }`, nil},
		{"malformed reject all", `{ rejectAll = "true" }`, nil},
		{"empty table", `{}`, nil},
	}
	parser, _ := newGradleVersionCatalogParser(gradleVersionCatalogMatcherConfig{})
	for _, tt := range tests {
		for _, referenced := range []bool{false, true} {
			name := tt.name + "/inline"
			content := "[libraries]\ncore = { module = \"org.example:core\", version = " + tt.version + " }\n"
			attributes := map[string]string{"alias": "core"}
			maps.Copy(attributes, tt.attributes)
			if referenced {
				name = tt.name + "/referenced"
				content = "[versions]\nshared = " + tt.version + "\n[libraries]\ncore = { module = \"org.example:core\", version.ref = \"shared\" }\n"
				attributes["version_ref"] = "shared"
			}
			t.Run(name, func(t *testing.T) {
				result, err := parser.Analyze("libs.versions.toml", []byte(content))
				if err != nil {
					t.Fatalf("Analyze: %v", err)
				}
				if result.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionPartial}) {
					t.Errorf("expected partial rich version extraction, got %+v", result.Analysis)
				}
				if len(result.Diagnostics) != 1 || result.Diagnostics[0].Severity != DiagnosticWarning {
					t.Errorf("expected a warning for rich version semantics, got %+v", result.Diagnostics)
				}
				if len(result.Dependencies) != 1 {
					t.Fatalf("expected known module to survive, got %+v", result.Dependencies)
				}
				dependency := result.Dependencies[0]
				if dependency.Name != "org.example:core" || dependency.Version != "" || dependency.VersionConstraint != "" {
					t.Errorf("rich version must not become a shared constraint: %+v", dependency)
				}
				if !maps.Equal(dependency.Attributes, attributes) {
					t.Errorf("attributes: got %v want %v", dependency.Attributes, attributes)
				}
				var gotRaw, wantRaw map[string]any
				if err := toml.Unmarshal([]byte(dependency.Raw), &gotRaw); err != nil {
					t.Fatalf("rich Raw must retain valid TOML: %v\n%s", err, dependency.Raw)
				}
				if err := toml.Unmarshal([]byte(content), &wantRaw); err != nil {
					t.Fatalf("parse test input: %v", err)
				}
				if !reflect.DeepEqual(gotRaw, wantRaw) {
					t.Errorf("rich Raw lost source fields: got %v want %v", gotRaw, wantRaw)
				}
			})
		}
	}
}

func TestGradleVersionCatalogParserKeepsAliasesForSameRichModule(t *testing.T) {
	parser, _ := newGradleVersionCatalogParser(gradleVersionCatalogMatcherConfig{})
	content := []byte(`[libraries]
second = { module = "org.example:core", version = { rejectAll = false } }
first = { module = "org.example:core", version = { prefer = "1.5" } }
`)
	for range 10 {
		result, err := parser.Analyze("libs.versions.toml", content)
		if err != nil {
			t.Fatalf("Analyze: %v", err)
		}
		aliases := make([]string, 0, len(result.Dependencies))
		for _, dependency := range result.Dependencies {
			aliases = append(aliases, dependency.Attributes["alias"])
		}
		if !slices.Equal(aliases, []string{"first", "second"}) {
			t.Fatalf("expected both aliases in stable order, got %v", aliases)
		}
	}
}

func TestGradleVersionCatalogParserKeepsSimpleVersions(t *testing.T) {
	parser, _ := newGradleVersionCatalogParser(gradleVersionCatalogMatcherConfig{})
	result, err := parser.Analyze("libs.versions.toml", []byte(`[versions]
shared = "1.5"
[libraries]
inline = { module = "org.example:inline", version = "1.5" }
reference = { module = "org.example:reference", version.ref = "shared" }
string = "org.example:string:1.5"
`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Analysis.Extraction != ExtractionComplete || len(result.Dependencies) != 3 || len(result.Diagnostics) != 0 {
		t.Fatalf("unexpected simple version extraction: %+v", result)
	}
	for _, dependency := range result.Dependencies {
		if dependency.VersionConstraint != "[1.5]" || dependency.Version != "" || dependency.Raw != dependency.Name+":1.5" {
			t.Errorf("simple version behavior changed: %+v", dependency)
		}
	}
}
