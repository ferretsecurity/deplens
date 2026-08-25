package analyze

import (
	"path/filepath"
	"testing"
)

func TestGithubActionsActionFixtures(t *testing.T) {
	tests := []struct {
		name     string
		fixture  string
		analysis SourceAnalysis
		want     []DependencyReference
	}{
		{
			name:     "node action without action dependencies",
			fixture:  "node-action-empty",
			analysis: SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete},
			want:     []DependencyReference{},
		},
		{
			name:     "docker action without action dependencies",
			fixture:  "docker-action-empty",
			analysis: SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete},
			want:     []DependencyReference{},
		},
		{
			name:     "composite action uses a pinned action",
			fixture:  "composite-action-uses",
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
			want: []DependencyReference{
				{PackageType: "github", Raw: "actions/upload-artifact@0123456789abcdef0123456789abcdef01234567", Name: "actions/upload-artifact", VersionConstraint: "0123456789abcdef0123456789abcdef01234567", SourceGroup: "runs.steps", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://github.com/actions/upload-artifact", "source_ref": "0123456789abcdef0123456789abcdef01234567"}},
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "github-actions", test.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "action.yaml")
			if source.Detector != "github-actions-action" || source.Analysis != test.analysis {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
		})
	}
}
