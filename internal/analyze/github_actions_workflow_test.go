package analyze

import "testing"

func TestGithubActionsWorkflowReferences(t *testing.T) {
	result, err := (githubActionsWorkflowParser{}).Analyze("ci.yaml", []byte(`
jobs:
  check:
    steps:
      - uses: acme/actions/setup@v2
      - uses: docker://ghcr.io/acme/tool:1.0
  release:
    uses: acme/ci/.github/workflows/release.yaml@release/stable
`))
	if err != nil {
		t.Fatal(err)
	}
	want := []DependencyReference{
		{Raw: "acme/actions/setup@v2", Name: "acme/actions/setup", VersionConstraint: "v2", SourceGroup: "jobs.check.steps", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_ref": "v2", "source_url": "https://github.com/acme/actions", "source_path": "setup"}},
		{Raw: "docker://ghcr.io/acme/tool:1.0", PackageType: "docker", Name: "ghcr.io/acme/tool", Version: "1.0", SourceGroup: "jobs.check.steps", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
		{Raw: "acme/ci/.github/workflows/release.yaml@release/stable", Name: "acme/ci/.github/workflows/release.yaml", VersionConstraint: "release/stable", SourceGroup: "jobs.release.uses", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://github.com/acme/ci", "source_ref": "release/stable", "source_path": ".github/workflows/release.yaml"}},
	}
	if !equalDependencies(result.Dependencies, want) {
		t.Fatalf("dependencies = %#v, want %#v", result.Dependencies, want)
	}
}
