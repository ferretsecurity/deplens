package analyze

import (
	"path/filepath"
	"testing"
)

func TestHelmChartLockFixturesExtractDependencyReferences(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []DependencyReference
	}{
		{
			name:    "HTTP repository dependency",
			fixture: "chart-lock-http-repository",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "nginx@8.8.4", Name: "nginx", Version: "8.8.4", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://charts.example.test/stable"}},
			},
		},
		{
			name:    "OCI registry dependency",
			fixture: "chart-lock-oci-registry",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "mongodb@19.1.16", Name: "mongodb", Version: "19.1.16", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "oci://registry.example.test/charts"}},
			},
		},
		{
			name:    "local chart dependency",
			fixture: "chart-lock-local-repository",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "chart-backend@0.1.0", Name: "chart-backend", Version: "0.1.0", SourceGroup: "dependencies", OriginKind: OriginPath, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_path": "./chart-backend"}},
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "helm", test.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "Chart.lock")
			if source.Detector != "helm-chart-lock" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
		})
	}
}

func TestHelmChartLockWithoutDependenciesIsCompleteAndAbsent(t *testing.T) {
	parser, err := newHelmChartLockParser(helmChartLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newHelmChartLockParser: %v", err)
	}
	result, err := parser.Analyze("Chart.lock", []byte("dependencies: []\ndigest: sha256:example\ngenerated: 2026-01-01T00:00:00Z\n"))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !result.Recognized || result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(result.Dependencies) != 0 {
		t.Fatalf("result = %+v", result)
	}
}
