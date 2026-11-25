package analyze

import (
	"path/filepath"
	"testing"
)

func TestGithubActionsWorkflows(t *testing.T) {
	result, err := Scan(filepath.Join("..", "..", "testdata", "github-actions", "workflows"), nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatal(err)
	}
	workflow := sourceForPath(t, result, ".github/workflows/package.yaml")
	if workflow.Detector != "github-actions-workflow" || workflow.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
		t.Fatalf("workflow = %+v", workflow)
	}
	want := map[string]string{
		"ghcr.io/acme/build:2@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef": "jobs.build.container.image",
		"postgres:16": "jobs.build.services.database.image",
		"node:22":     "jobs.test.container",
		"actions/checkout@11d5960a326750d5838078e36cf38b85af677262": "jobs.build.steps",
		"acme/tools/setup@v2":                                      "jobs.build.steps",
		"docker://ghcr.io/acme/tool:1.0":                           "jobs.build.steps",
		"acme/shared/.github/workflows/release.yml@release/stable": "jobs.release.uses",
	}
	for _, dep := range workflow.Dependencies {
		if group, ok := want[dep.Raw]; !ok || dep.SourceGroup != group {
			t.Fatalf("unexpected dependency %+v", dep)
		}
		delete(want, dep.Raw)
	}
	if len(want) > 0 {
		t.Fatalf("missing dependencies: %v", want)
	}
	local := sourceForPath(t, result, ".github/workflows/local.yml")
	if local.Detector != "github-actions-workflow" || local.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(local.Dependencies) != 0 {
		t.Fatalf("local workflow = %+v", local)
	}
}
