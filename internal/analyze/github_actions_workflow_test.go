package analyze

import (
	"fmt"
	"strings"
	"testing"
)

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

func TestGithubActionsWorkflowContainerImages(t *testing.T) {
	result, err := (githubActionsWorkflowParser{}).Analyze("ci.yaml", []byte(`
jobs:
  build:
    container: node:22
    services:
      database:
        image: postgres:16
        env:
          IMAGE: ignored:1
      cache:
        image: redis
  release:
    container:
      image: localhost:5000/acme/tool:2@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
      credentials:
        username: ignored
        password: ignored
      env:
        IMAGE: ignored:2
    services:
      database:
        image: postgres:16
`))
	if err != nil {
		t.Fatal(err)
	}
	want := []DependencyReference{
		{PackageType: "docker", Raw: "node:22", Name: "node", Version: "22", SourceGroup: "jobs.build.container", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
		{PackageType: "docker", Raw: "redis", Name: "redis", SourceGroup: "jobs.build.services.cache.image", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
		{PackageType: "docker", Raw: "postgres:16", Name: "postgres", Version: "16", SourceGroup: "jobs.build.services.database.image", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
		{PackageType: "docker", Raw: "localhost:5000/acme/tool:2@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", Name: "localhost:5000/acme/tool", Version: "2", SourceGroup: "jobs.release.container.image", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"digest": "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}},
		{PackageType: "docker", Raw: "postgres:16", Name: "postgres", Version: "16", SourceGroup: "jobs.release.services.database.image", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
	}
	if result.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) || len(result.Diagnostics) != 0 || !equalDependencies(result.Dependencies, want) {
		t.Fatalf("result = %+v, want dependencies %#v", result, want)
	}
}

func TestGithubActionsWorkflowInvalidImagesKeepOtherReferences(t *testing.T) {
	for _, declaration := range []string{
		"container: ${{ matrix.image }}",
		"container: {image: 'node:${{ matrix.version }}'}",
		"container: [node:22]",
		"container: {image: [node:22]}",
		"container: {image: true}",
		"container: {options: --privileged}",
		"services: []",
		"services: {database: postgres:16}",
		"services: {database: {image: '${{ matrix.image }}'}}",
		"services: {database: {image: 123}}",
		"services: {database: {image: null}}",
		"services: {database: {env: {IMAGE: ignored:1}}}",
	} {
		t.Run(declaration, func(t *testing.T) {
			result, err := (githubActionsWorkflowParser{}).Analyze("ci.yaml", []byte("jobs:\n  build:\n    "+declaration+"\n    steps:\n      - uses: actions/checkout@v4\n"))
			if err != nil {
				t.Fatal(err)
			}
			if result.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionPartial}) || len(result.Dependencies) != 1 || result.Dependencies[0].Raw != "actions/checkout@v4" {
				t.Fatalf("result = %+v", result)
			}
			if len(result.Diagnostics) != 1 || result.Diagnostics[0].Severity != DiagnosticWarning || !strings.Contains(result.Diagnostics[0].Message, "jobs.build.") {
				t.Fatalf("diagnostics = %+v", result.Diagnostics)
			}
		})
	}
}

func TestGithubActionsWorkflowMalformedImageReferences(t *testing.T) {
	for _, image := range []string{
		"https://registry.example.com/acme/tool:1",
		"ghcr.io/acme/tool:",
		"acme//tool:1",
		"Acme/Tool:1",
		"tool:bad/tag",
		"tool@sha256:abc123",
		"tool@sha256:" + strings.Repeat("g", 64),
		"tool@sha256:" + strings.Repeat("a", 64) + "@sha256:" + strings.Repeat("b", 64),
		"tool\nbogus:1",
	} {
		t.Run(image, func(t *testing.T) {
			content := "jobs:\n  build:\n    container:\n      image: " + fmt.Sprintf("%q", image) + "\n    services:\n      cache:\n        image: redis:7\n"
			result, err := (githubActionsWorkflowParser{}).Analyze("ci.yaml", []byte(content))
			if err != nil {
				t.Fatal(err)
			}
			if result.Analysis.Extraction != ExtractionPartial || len(result.Dependencies) != 1 || result.Dependencies[0].Name != "redis" || len(result.Diagnostics) != 1 || !strings.Contains(result.Diagnostics[0].Message, "jobs.build.container.image") {
				t.Fatalf("result = %+v", result)
			}
		})
	}
}

func TestGithubActionsWorkflowOnlyDynamicImages(t *testing.T) {
	result, err := (githubActionsWorkflowParser{}).Analyze("ci.yaml", []byte(`
jobs:
  build:
    container: ${{ matrix.image }}
    services:
      database:
        image: postgres:${{ inputs.version }}
`))
	if err != nil {
		t.Fatal(err)
	}
	if result.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionUnsupported}) || len(result.Dependencies) != 0 || len(result.Diagnostics) != 2 {
		t.Fatalf("result = %+v", result)
	}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Severity != DiagnosticWarning || !strings.Contains(diagnostic.Message, "unresolved interpolation") {
			t.Fatalf("diagnostic = %+v", diagnostic)
		}
	}
}
