package analyze

import (
	"path/filepath"
	"testing"
)

func TestJSONNetLockFixturesExtractDependencyReferences(t *testing.T) {
	tests := []struct {
		name     string
		fixture  string
		analysis SourceAnalysis
		want     []DependencyReference
	}{
		{
			name:     "named Git dependency without checksum",
			fixture:  "lock-named-git-no-sum",
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
			want: []DependencyReference{
				{PackageType: "generic", Raw: "dashboards@0123456789abcdef0123456789abcdef01234567", Name: "dashboards", Version: "0123456789abcdef0123456789abcdef01234567", SourceGroup: "dependencies", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://example.test/observability/dashboards", "source_path": "jsonnet", "source_ref": "0123456789abcdef0123456789abcdef01234567", "source_ref_kind": "commit"}},
			},
		},
		{
			name:     "unnamed Git dependencies with checksums",
			fixture:  "lock-unnamed-git-with-sums",
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
			want: []DependencyReference{
				{PackageType: "generic", Raw: "alerts@1111111111111111111111111111111111111111", Name: "alerts", Version: "1111111111111111111111111111111111111111", SourceGroup: "dependencies", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://example.test/observability/alerts.git", "source_path": "lib", "source_ref": "1111111111111111111111111111111111111111", "source_ref_kind": "commit", "checksum": "sum-alerts"}},
				{PackageType: "generic", Raw: "widgets@2222222222222222222222222222222222222222", Name: "widgets", Version: "2222222222222222222222222222222222222222", SourceGroup: "dependencies", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://example.test/observability/widgets.git", "source_path": "builder", "source_ref": "2222222222222222222222222222222222222222", "source_ref_kind": "commit", "checksum": "sum-widgets"}},
			},
		},
		{
			name:     "mixed named and unnamed Git dependencies",
			fixture:  "lock-mixed-named-unnamed-git",
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
			want: []DependencyReference{
				{PackageType: "generic", Raw: "common@3333333333333333333333333333333333333333", Name: "common", Version: "3333333333333333333333333333333333333333", SourceGroup: "dependencies", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://example.test/team/common.git", "source_ref": "3333333333333333333333333333333333333333", "source_ref_kind": "commit", "checksum": "sum-common"}},
				{PackageType: "generic", Raw: "rendering@4444444444444444444444444444444444444444", Name: "rendering", Version: "4444444444444444444444444444444444444444", SourceGroup: "dependencies", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://example.test/team/rendering.git", "source_path": "latest", "source_ref": "4444444444444444444444444444444444444444", "source_ref_kind": "commit", "checksum": "sum-latest"}},
				{PackageType: "generic", Raw: "rendering@5555555555555555555555555555555555555555", Name: "rendering", Version: "5555555555555555555555555555555555555555", SourceGroup: "dependencies", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://example.test/team/rendering.git", "source_path": "legacy", "source_ref": "5555555555555555555555555555555555555555", "source_ref_kind": "commit", "checksum": "sum-legacy"}},
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "jsonnet", test.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "jsonnetfile.lock.json")
			if source.Detector != "jsonnet-lock" || source.Analysis != test.analysis {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
		})
	}
}

func TestJSONNetLockEmptyDependenciesAreAbsent(t *testing.T) {
	parser, err := newJSONNetLockParser(jsonnetLockParserConfig{})
	if err != nil {
		t.Fatalf("newJSONNetLockParser: %v", err)
	}
	result, err := parser.Analyze("jsonnetfile.lock.json", []byte(`{"version": 1, "dependencies": []}`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !result.Recognized || result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(result.Dependencies) != 0 {
		t.Fatalf("result = %+v", result)
	}
}
