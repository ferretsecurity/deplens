package analyze

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestIvySemanticFixtures(t *testing.T) {
	tests := []struct {
		name            string
		fixture         string
		wantRaw         []string
		wantConstraints []string
	}{
		{
			name:            "exact revisions",
			fixture:         "ivy-exact-revisions",
			wantRaw:         []string{"example.metrics:counter:1.4.0", "example.net:socket:3.2.1", "example.test:assertions:5.0"},
			wantConstraints: []string{"[1.4.0]", "[3.2.1]", "[5.0]"},
		},
		{
			name:            "property revision with classifier-style configurations",
			fixture:         "ivy-property-configurations",
			wantRaw:         []string{"example.graphics:render:${render.version}", "example.graphics:render:${render.version}"},
			wantConstraints: []string{"[${render.version}]", "[${render.version}]"},
		},
		{
			name:            "configuration scoped dependencies",
			fixture:         "ivy-scoped-dependencies",
			wantRaw:         []string{"example.build:reporting:2.1", "example.logging:api:4.0.0", "example.testing:runner:6.3.0"},
			wantConstraints: []string{"[2.1]", "[4.0.0]", "[6.3.0]"},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "java", tc.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "ivy.xml")
			if source.Detector != "java-ivy" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			gotRaw := dependencyNames(source.Dependencies)
			if !slices.Equal(gotRaw, tc.wantRaw) {
				t.Fatalf("dependency raw values = %v, want %v", gotRaw, tc.wantRaw)
			}
			gotConstraints := make([]string, 0, len(source.Dependencies))
			for _, dependency := range source.Dependencies {
				gotConstraints = append(gotConstraints, dependency.VersionConstraint)
				if dependency.PackageType != "maven" || dependency.Name == "" || dependency.SourceGroup != "dependencies" ||
					dependency.OriginKind != OriginRegistry || dependency.Relationship != RelationshipDirect || dependency.Scope != ScopeRuntime {
					t.Fatalf("dependency = %+v", dependency)
				}
			}
			if !slices.Equal(gotConstraints, tc.wantConstraints) {
				t.Fatalf("dependency constraints = %v, want %v", gotConstraints, tc.wantConstraints)
			}
			switch tc.fixture {
			case "ivy-property-configurations":
				configured := 0
				for _, dependency := range source.Dependencies {
					if dependency.Attributes["conf"] == "docs;sources" {
						configured++
					}
				}
				if configured != 1 {
					t.Fatalf("dependencies = %+v, want one docs/sources mapping", source.Dependencies)
				}
			case "ivy-scoped-dependencies":
				if dependencyByName(t, source.Dependencies, "example.build:reporting").Attributes["conf"] != "reports" ||
					dependencyByName(t, source.Dependencies, "example.testing:runner").Attributes["conf"] != "test" {
					t.Fatalf("dependencies = %+v, want reporting and test mappings", source.Dependencies)
				}
			}
		})
	}
}

func TestIvyParserReportsAbsentForManifestWithoutDependencies(t *testing.T) {
	parser, err := newIvyParser(ivyMatcherConfig{})
	if err != nil {
		t.Fatalf("newIvyParser: %v", err)
	}
	result, err := parser.Analyze("ivy.xml", []byte(`<ivy-module version="2.0"><info organisation="example" module="empty"/></ivy-module>`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(result.Dependencies) != 0 {
		t.Fatalf("result = %+v", result)
	}
}
