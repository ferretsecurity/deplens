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

func TestGradleBuildParserPreservesVersionlessCoordinates(t *testing.T) {
	tests := []struct {
		path    string
		content string
	}{
		{
			path: "build.gradle",
			content: `dependencies {
  implementation platform('org.example:platform:1.2.3')
  implementation 'org.example:core'
  testImplementation('org.example:test-kit')
  compileOnly 'org.example:annotations'
}`,
		},
		{
			path: "build.gradle.kts",
			content: `dependencies {
  implementation(platform("org.example:platform:1.2.3"))
  implementation("org.example:core")
  testImplementation("org.example:test-kit")
  compileOnly("org.example:annotations")
}`,
		},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			parser, _ := newGradleBuildParser(gradleBuildMatcherConfig{})
			result, err := parser.Analyze(test.path, []byte(test.content))
			if err != nil {
				t.Fatalf("Analyze: %v", err)
			}
			if result.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) || len(result.Diagnostics) != 0 {
				t.Fatalf("unexpected result: %+v", result)
			}
			if got, want := dependencyNames(result.Dependencies), []string{
				"org.example:annotations",
				"org.example:core",
				"org.example:platform:1.2.3",
				"org.example:test-kit",
			}; !slices.Equal(got, want) {
				t.Fatalf("dependencies: got %v want %v", got, want)
			}
			for _, dependency := range result.Dependencies {
				if dependency.Name == "org.example:platform" {
					if dependency.SourceGroup != "implementation" || dependency.Scope != ScopeRuntime || dependency.VersionConstraint != "[1.2.3]" {
						t.Fatalf("unexpected platform dependency: %+v", dependency)
					}
					continue
				}
				if dependency.Raw != dependency.Name || dependency.Version != "" || dependency.VersionConstraint != "" || dependency.VERS != "" || dependency.Relationship != RelationshipDirect {
					t.Fatalf("unexpected versionless dependency: %+v", dependency)
				}
				wantScope := map[string]DependencyScope{
					"compileOnly":        ScopeBuild,
					"implementation":     ScopeRuntime,
					"testImplementation": ScopeTest,
				}[dependency.SourceGroup]
				if wantScope == "" || dependency.Scope != wantScope {
					t.Fatalf("unexpected configuration or scope: %+v", dependency)
				}
			}
		})
	}
}

func TestGradleBuildParserReportsRichVersionClosuresWithoutPhantomDependencies(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		content string
	}{
		{
			name: "Groovy literals",
			path: "build.gradle",
			content: `dependencies {
  implementation('org.example:core') {
    version {
      strictly '[1.0,2.0['
      prefer 'vendor:preview'
      reject 'org.example:phantom:1.6'
    }
  }
}`,
		},
		{
			name: "Kotlin literals",
			path: "build.gradle.kts",
			content: `dependencies {
  implementation("org.example:core") {
    version {
      strictly("[1.0,2.0[")
      prefer("vendor:preview")
      reject("org.example:phantom:1.6")
    }
  }
}`,
		},
		{
			name: "dynamic closure",
			path: "build.gradle.kts",
			content: `dependencies {
  implementation("org.example:core") {
    version {
      require(selectedVersion)
      if (preview) { prefer("vendor:preview") }
    }
  }
}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parser, _ := newGradleBuildParser(gradleBuildMatcherConfig{})
			result, err := parser.Analyze(test.path, []byte(test.content))
			if err != nil {
				t.Fatalf("Analyze: %v", err)
			}
			if result.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionPartial}) {
				t.Fatalf("unexpected analysis: %+v", result.Analysis)
			}
			if got, want := dependencyNames(result.Dependencies), []string{"org.example:core"}; !slices.Equal(got, want) {
				t.Fatalf("dependencies: got %v want %v", got, want)
			}
			dependency := result.Dependencies[0]
			if dependency.Name != "org.example:core" || dependency.SourceGroup != "implementation" || dependency.Scope != ScopeRuntime || dependency.Version != "" || dependency.VersionConstraint != "" {
				t.Fatalf("unexpected dependency: %+v", dependency)
			}
			if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != incompleteExtractionCode || !strings.Contains(result.Diagnostics[0].Message, "rich version") {
				t.Fatalf("unexpected diagnostics: %+v", result.Diagnostics)
			}
		})
	}
}

func TestGradleBuildParserIgnoresRichVersionTextInsideStrings(t *testing.T) {
	parser, _ := newGradleBuildParser(gradleBuildMatcherConfig{})
	result, err := parser.Analyze("build.gradle", []byte(`dependencies {
  implementation("org.example:core") {
    because("a version { marker")
    because('an escaped \' version { marker')
  }
}`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) || len(result.Diagnostics) != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if got, want := dependencyNames(result.Dependencies), []string{"org.example:core"}; !slices.Equal(got, want) {
		t.Fatalf("dependencies: got %v want %v", got, want)
	}
}

func TestGradleBuildParserIgnoresDependencyConfigurationMetadata(t *testing.T) {
	for _, path := range []string{"build.gradle", "build.gradle.kts"} {
		t.Run(path, func(t *testing.T) {
			parser, _ := newGradleBuildParser(gradleBuildMatcherConfig{})
			result, err := parser.Analyze(path, []byte(`dependencies {
  customConfiguration("org.example:core") {
    because("reason: detail")
    because("reason:detail")
    because("reason:detail:more")
    capabilities {
      requireCapability("org.example:core-fixtures")
      requireCapabilities("org.example:core-extra")
    }
  }
  constraints {
    implementation("org.example:constraint")
  }
}`))
			if err != nil {
				t.Fatalf("Analyze: %v", err)
			}
			if got, want := dependencyNames(result.Dependencies), []string{"org.example:core", "org.example:constraint"}; !slices.Equal(got, want) {
				t.Fatalf("dependencies: got %v want %v", got, want)
			}
			if result.Analysis.Extraction != ExtractionComplete || len(result.Diagnostics) != 0 || result.Dependencies[0].SourceGroup != "customConfiguration" {
				t.Fatalf("unexpected result: %+v", result)
			}
		})
	}
}

func TestGradleBuildParserIgnoresMetadataOnOtherDeclarationForms(t *testing.T) {
	for _, declaration := range []string{
		`implementation(group: 'org.example', name: 'core', version: '1.0')`,
		`implementation group: 'org.example', name: 'core', version: '1.0'`,
		`implementation(group = "org.example", name = "core", version = "1.0")`,
		`implementation(libs.example.core)`,
		`implementation(project(":core"))`,
	} {
		t.Run(declaration, func(t *testing.T) {
			parser, _ := newGradleBuildParser(gradleBuildMatcherConfig{})
			result, err := parser.Analyze("build.gradle", []byte("dependencies {\n"+declaration+` {
  because("reason: detail")
  capabilities {
    requireCapability("org.example:core-fixtures")
  }
}

}`))
			if err != nil {
				t.Fatalf("Analyze: %v", err)
			}
			for _, dependency := range result.Dependencies {
				if dependency.Name != "org.example:core" {
					t.Fatalf("unexpected metadata reference: %+v", dependency)
				}
			}
			wantCount := 0
			if strings.Contains(declaration, "group") {
				wantCount = 1
			}
			if len(result.Dependencies) != wantCount {
				t.Fatalf("dependencies: got %+v want %d references", result.Dependencies, wantCount)
			}
		})
	}
}

func TestGradleBuildParserPreservesDependencyContainers(t *testing.T) {
	parser, _ := newGradleBuildParser(gradleBuildMatcherConfig{})
	result, err := parser.Analyze("build.gradle.kts", []byte(`dependencies {
  implementation("org.example:first:1.0"); constraints {
    implementation("org.example:constraint:2.0")
  }
  if (enabled) {
    implementation("org.example:conditional:1.0")
  }
  else if (fallback) {
    implementation("org.example:fallback:1.0")
  }
  try {
    implementation("org.example:try:1.0")
  }
  catch (ignored: Exception) {
    implementation("org.example:catch:1.0")
  }
  implementation
  ("org.example:split") {
    because("reason: detail")
  }
}`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if got, want := dependencyNames(result.Dependencies), []string{
		"org.example:catch:1.0", "org.example:conditional:1.0", "org.example:constraint:2.0",
		"org.example:fallback:1.0", "org.example:first:1.0", "org.example:split", "org.example:try:1.0",
	}; !slices.Equal(got, want) {
		t.Fatalf("dependencies: got %v want %v", got, want)
	}
}

func TestGradleBuildParserPreservesExecutableContainers(t *testing.T) {
	for _, container := range []string{"true ->", "with(this)", "listOf(1, 2).forEach"} {
		t.Run(container, func(t *testing.T) {
			parser, _ := newGradleBuildParser(gradleBuildMatcherConfig{})
			body := container + ` {
  implementation("org.example:contained:1.0")
}`
			if container == "true ->" {
				body = "when {\n" + body + "\n}"
			}
			result, err := parser.Analyze("build.gradle.kts", []byte("dependencies {\n"+body+"\n}"))
			if err != nil {
				t.Fatalf("Analyze: %v", err)
			}
			if got, want := dependencyNames(result.Dependencies), []string{"org.example:contained:1.0"}; !slices.Equal(got, want) {
				t.Fatalf("dependencies: got %v want %v", got, want)
			}
		})
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
