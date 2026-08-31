package analyze

import (
	"path/filepath"
	"testing"
)

func TestPantsConfigFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []DependencyReference
	}{
		{
			name:    "Python backends",
			fixture: "python-backends",
			want: []DependencyReference{
				pantsBackendTestDependency("example.pants.python", "GLOBAL.backend_packages"),
				pantsBackendTestDependency("example.pants.python.lint", "GLOBAL.backend_packages"),
				pantsVersionTestDependency("2.99.1"),
			},
		},
		{
			name:    "JVM backends",
			fixture: "jvm-backends",
			want: []DependencyReference{
				pantsBackendTestDependency("example.pants.java", "GLOBAL.backend_packages"),
				pantsBackendTestDependency("example.pants.java.format", "GLOBAL.backend_packages"),
				pantsVersionTestDependency("2.99.2"),
			},
		},
		{
			name:    "Go backends",
			fixture: "go-backends",
			want: []DependencyReference{
				pantsBackendTestDependency("example.pants.go", "GLOBAL.backend_packages"),
				pantsBackendTestDependency("example.pants.go.lint", "GLOBAL.backend_packages"),
				pantsVersionTestDependency("2.99.3"),
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "pants", test.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "pants.toml")
			wantAnalysis := SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}
			if source.Detector != "pants-config" || source.Analysis != wantAnalysis {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
		})
	}
}

func TestPantsConfigWithoutDependencyReferencesIsCompleteAndAbsent(t *testing.T) {
	result, err := pantsConfigParser{}.Analyze("pants.toml", []byte("[GLOBAL]\ncolors = true\n"))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) {
		t.Fatalf("analysis = %+v", result.Analysis)
	}
	if len(result.Dependencies) != 0 {
		t.Fatalf("dependencies = %#v, want none", result.Dependencies)
	}
}

func pantsVersionTestDependency(version string) DependencyReference {
	return DependencyReference{
		PackageType:       "pypi",
		Raw:               pantsDistribution + "@" + version,
		Name:              pantsDistribution,
		VersionConstraint: version,
		VERS:              dependencyVERS("pypi", version),
		SourceGroup:       "GLOBAL.pants_version",
		OriginKind:        OriginRegistry,
		Relationship:      RelationshipDirect,
		Scope:             ScopeBuild,
	}
}

func pantsBackendTestDependency(name, sourceGroup string) DependencyReference {
	return DependencyReference{
		PackageType:  "pypi",
		Raw:          name,
		Name:         name,
		SourceGroup:  sourceGroup,
		OriginKind:   OriginRegistry,
		Relationship: RelationshipDirect,
		Scope:        ScopeBuild,
	}
}
