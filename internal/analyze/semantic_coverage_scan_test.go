package analyze

import (
	"path/filepath"
	"testing"
)

func TestDefaultRulesExtractCommonSemanticCoverageFixtures(t *testing.T) {
	tests := []struct {
		name         string
		fixture      string
		path         string
		detector     DetectorID
		dependencies int
		extraction   ExtractionState
	}{
		{
			name:         "package.json manifest",
			fixture:      filepath.Join("javascript", "package-json-versatile"),
			path:         "package.json",
			detector:     "js",
			dependencies: 7,
			extraction:   ExtractionComplete,
		},
		{
			name:         "Gradle Groovy build",
			fixture:      filepath.Join("java", "gradle-build"),
			path:         "build.gradle",
			detector:     "java-gradle",
			dependencies: 2,
			extraction:   ExtractionComplete,
		},
		{
			name:         "Gradle Kotlin build with alias",
			fixture:      filepath.Join("java", "gradle-build-kts"),
			path:         "build.gradle.kts",
			detector:     "java-gradle-kts",
			dependencies: 1,
			extraction:   ExtractionPartial,
		},
		{
			name:         "Gradle lockfile",
			fixture:      filepath.Join("java", "gradle-lock"),
			path:         "gradle.lockfile",
			detector:     "java-gradle-lockfile",
			dependencies: 2,
			extraction:   ExtractionComplete,
		},
		{
			name:         "Gradle version catalog",
			fixture:      filepath.Join("java", "gradle-version-catalog"),
			path:         filepath.ToSlash(filepath.Join("gradle", "libs.versions.toml")),
			detector:     "java-gradle-version-catalog",
			dependencies: 1,
			extraction:   ExtractionComplete,
		},
		{
			name:         "Gemfile",
			fixture:      filepath.Join("ruby", "gemfile"),
			path:         "Gemfile",
			detector:     "ruby-gemfile",
			dependencies: 2,
			extraction:   ExtractionComplete,
		},
		{
			name:         "Gemfile lock",
			fixture:      filepath.Join("ruby", "gemfile-lock"),
			path:         "Gemfile.lock",
			detector:     "ruby-gemfile-lock",
			dependencies: 2,
			extraction:   ExtractionComplete,
		},
		{
			name:         "Dockerfile",
			fixture:      filepath.Join("docker", "dockerfile"),
			path:         "Dockerfile",
			detector:     "docker-dockerfile",
			dependencies: 1,
			extraction:   ExtractionComplete,
		},
		{
			name:         "Docker Compose",
			fixture:      filepath.Join("docker", "compose-v2"),
			path:         "compose.yaml",
			detector:     "docker-compose",
			dependencies: 1,
			extraction:   ExtractionComplete,
		},
		{
			name:         "Maven POM",
			fixture:      filepath.Join("java", "maven-pom-semantic"),
			path:         "pom.xml",
			detector:     "java",
			dependencies: 3,
			extraction:   ExtractionComplete,
		},
		{
			name:         "Cargo manifest",
			fixture:      filepath.Join("rust", "cargo-manifest-semantic"),
			path:         "Cargo.toml",
			detector:     "rust-cargo",
			dependencies: 5,
			extraction:   ExtractionComplete,
		},
		{
			name:         "Composer manifest",
			fixture:      filepath.Join("php", "composer-json-semantic"),
			path:         "composer.json",
			detector:     "php-composer",
			dependencies: 2,
			extraction:   ExtractionComplete,
		},
		{
			name:         ".NET central package catalog",
			fixture:      filepath.Join("dotnet", "central-packages-semantic"),
			path:         "Directory.Packages.props",
			detector:     "dotnet-directory-packages-props",
			dependencies: 1,
			extraction:   ExtractionComplete,
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := filepath.Join("..", "..", "testdata", test.fixture)
			result, err := Scan(root, nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			var source *DependencySourceResult
			for index := range result.Sources {
				if result.Sources[index].Path == test.path {
					source = &result.Sources[index]
					break
				}
			}
			if source == nil {
				t.Fatalf("source %q not found in %+v", test.path, result.Sources)
			}
			if source.Detector != test.detector || len(source.Dependencies) != test.dependencies || source.Analysis.Extraction != test.extraction {
				t.Fatalf("unexpected source: %+v", *source)
			}
		})
	}
}
