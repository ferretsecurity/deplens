package analyze

import (
	"path/filepath"
	"testing"
)

func TestGithubActionsActionFixtures(t *testing.T) {
	tests := []struct {
		name           string
		fixture        string
		analysis       SourceAnalysis
		want           []DependencyReference
		wantDiagnostic string
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
		{
			name:     "composite action uses a tagged Docker image",
			fixture:  "composite-action-docker-tag",
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
			want: []DependencyReference{
				{PackageType: "docker", Raw: "docker://snyk/snyk:scala", Name: "snyk/snyk", Version: "scala", SourceGroup: "runs.steps", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
			},
		},
		{
			name:           "composite action uses a dynamic Docker image",
			fixture:        "composite-action-docker-dynamic",
			analysis:       SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionUnsupported},
			want:           []DependencyReference{},
			wantDiagnostic: "runs.steps[0].uses could not be parsed as a Docker image: docker://registry.example.test/acme/scanner:${{inputs.tag}}",
		},
		{
			name:     "composite action mixes a pinned action and a Docker digest",
			fixture:  "composite-action-mixed-uses",
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
			want: []DependencyReference{
				{PackageType: "github", Raw: "actions/upload-artifact@0123456789abcdef0123456789abcdef01234567", Name: "actions/upload-artifact", VersionConstraint: "0123456789abcdef0123456789abcdef01234567", SourceGroup: "runs.steps", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://github.com/actions/upload-artifact", "source_ref": "0123456789abcdef0123456789abcdef01234567"}},
				{PackageType: "docker", Raw: "docker://registry.example.test/acme/scanner@sha256:0123456789abcdef", Name: "registry.example.test/acme/scanner", SourceGroup: "runs.steps", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"digest": "sha256:0123456789abcdef"}},
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
			if test.wantDiagnostic == "" {
				if len(source.Diagnostics) != 0 {
					t.Fatalf("diagnostics = %#v, want none", source.Diagnostics)
				}
				return
			}
			if len(source.Diagnostics) != 1 || source.Diagnostics[0] != (Diagnostic{Severity: DiagnosticWarning, Code: incompleteExtractionCode, Message: test.wantDiagnostic}) {
				t.Fatalf("diagnostics = %#v, want warning %q", source.Diagnostics, test.wantDiagnostic)
			}
		})
	}
}
