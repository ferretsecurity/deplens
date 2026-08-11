package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitializeProgressWritesV2ReviewedPlans(t *testing.T) {
	projectRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "collection.yaml")
	var stdout, stderr bytes.Buffer
	if got := run([]string{"initialize-progress", "--progress", path}, projectRoot, &stdout, &stderr, unavailableResearcher{}); got != 0 {
		t.Fatalf("initialize exit status = %d, stderr = %s", got, stderr.String())
	}
	p, err := readProgress(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Version != progressVersion || p.Limits != defaultCollectionLimits {
		t.Fatalf("progress = %+v", p)
	}
	ready, review := 0, 0
	for _, detector := range p.Detectors {
		switch detector.State {
		case stateNeedsQueryReview:
			review++
			if len(detector.QueryPlan) != 0 {
				t.Fatalf("query-review detector has plan: %+v", detector)
			}
		case statePending:
			ready++
			if len(detector.QueryPlan) == 0 || !isSortedUnique(detector.QueryPlan) {
				t.Fatalf("ready detector has noncanonical plan: %+v", detector)
			}
		default:
			t.Fatalf("unexpected initialized state: %+v", detector)
		}
	}
	if ready != 142 || review != 2 {
		t.Fatalf("plans = %d ready, %d review", ready, review)
	}
	if err := writeProgress(path, p); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeProgress(path, p); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("progress did not round-trip canonically")
	}
}

func TestReadProgressRejectsOldAndInvalidV2Documents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "collection.yaml")
	const validLimits = "limits:\n  queries: 8\n  result_pages: 10\n  candidate_inspections: 40\n  decoded_response_bytes: 16777216\n  packet_tokens: 50000\n  selector_invocations: 2\n  source_bytes: 2097152\n  valid_iterations: 7\n"
	tests := []struct {
		name     string
		document string
		wantText string
	}{
		{"old version", "version: 1\ndetectors: []\n", "initialize a fresh collection"},
		{"unknown field", "version: 2\nlimits:\n  queries: 8\nunknown: true\ndetectors: []\n", ""},
		{"obsolete log path", "version: 2\nlimits:\n  queries: 8\nlog_path: .deplens/collector.jsonl\ndetectors: []\n", ""},
		{"invalid query review", "version: 2\nlimits:\n  queries: 8\ndetectors:\n  - id: example\n    state: needs-query-review\n    query_plan: [filename:x]\n", ""},
		{"obsolete blocked state", "version: 2\n" + validLimits + "detectors:\n  - id: example\n    state: blocked\n    iterations: 7\n    query_plan: [filename:x]\n", ""},
		{"obsolete excluded state", "version: 2\n" + validLimits + "detectors:\n  - id: example\n    state: excluded\n    query_plan: [filename:x]\n", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(test.document), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := readProgress(path)
			if err == nil {
				t.Fatalf("readProgress accepted %q", test.document)
			}
			if test.wantText != "" && !strings.Contains(err.Error(), test.wantText) {
				t.Fatalf("readProgress error = %v, want text %q", err, test.wantText)
			}
		})
	}
}

func TestRunRejectsObsoleteRawLogRetentionFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if got := run([]string{"run", "--retain-logs"}, t.TempDir(), &stdout, &stderr, unavailableResearcher{}); got != 1 {
		t.Fatalf("run exit status = %d, want 1", got)
	}
	if !strings.Contains(stderr.String(), "flag provided but not defined") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestProvenanceRejectsLegacySchemaAndMetadata(t *testing.T) {
	if err := validateProvenance(Provenance{Version: 1}, "example"); err == nil {
		t.Fatal("version-1 provenance was accepted")
	}
	for _, document := range []string{
		"version: 2\nproject_kind: library\n",
		"version: 2\nvariation_tags: [common]\n",
		"version: 2\nmodel_path: source/file\n",
	} {
		if _, err := parseProvenance(document); err == nil {
			t.Fatalf("legacy provenance metadata was accepted: %q", document)
		}
	}
}

func TestWriteProgressRejectsLegacyOrIncompleteProgress(t *testing.T) {
	path := filepath.Join(t.TempDir(), "collection.yaml")
	for _, test := range []struct {
		name     string
		progress Progress
	}{
		{
			name:     "legacy version",
			progress: Progress{Version: 1, Detectors: []DetectorProgress{{ID: "example", State: statePending}}},
		},
		{
			name:     "missing required v2 fields",
			progress: Progress{Version: progressVersion, Detectors: []DetectorProgress{{ID: "example", State: statePending}}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := writeProgress(path, test.progress); err == nil {
				t.Fatalf("writeProgress accepted %+v", test.progress)
			}
		})
	}
}

func TestWriteProgressSyncsParentDirectoryAfterAtomicInstall(t *testing.T) {
	path := filepath.Join(t.TempDir(), "collection.yaml")
	called := ""
	originalSync := syncProgressDirectory
	syncProgressDirectory = func(directory string) error {
		called = directory
		return nil
	}
	t.Cleanup(func() { syncProgressDirectory = originalSync })

	if err := writeProgress(path, validTestProgress(DetectorProgress{ID: "example", State: statePending})); err != nil {
		t.Fatal(err)
	}
	if called != filepath.Dir(path) {
		t.Fatalf("synced directory = %q, want %q", called, filepath.Dir(path))
	}
}

func TestWriteProgressReturnsDirectorySyncError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "collection.yaml")
	syncErr := errors.New("sync directory")
	originalSync := syncProgressDirectory
	syncProgressDirectory = func(string) error { return syncErr }
	t.Cleanup(func() { syncProgressDirectory = originalSync })

	err := writeProgress(path, validTestProgress(DetectorProgress{ID: "example", State: statePending}))
	if !errors.Is(err, syncErr) {
		t.Fatalf("writeProgress error = %v, want %v", err, syncErr)
	}
}

func TestQueryReviewDoesNotBlockCollection(t *testing.T) {
	detectors := []DetectorProgress{{ID: "review", State: stateNeedsQueryReview}, {ID: "ready", State: statePending, QueryPlan: []string{"filename:ready"}}}
	if got := selectDetector(detectors, ""); got == nil || got.ID != "ready" {
		t.Fatalf("selected = %+v", got)
	}
	if got := collectionSummary(Progress{Detectors: detectors}, io.Discard); got != 0 {
		t.Fatalf("summary exit = %d", got)
	}
}

func TestCollectionSummaryReportsNonBlockingReviewStates(t *testing.T) {
	var stdout bytes.Buffer
	p := Progress{Detectors: []DetectorProgress{
		{ID: "content", State: stateNeedsContentReview},
		{ID: "collection", State: stateNeedsCollectionReview},
		{ID: "ready", State: statePending, QueryPlan: []string{"filename:ready"}},
	}}
	if got := collectionSummary(p, &stdout); got != 0 {
		t.Fatalf("summary exit code = %d", got)
	}
	for _, want := range []string{"1 needs content review (content)", "1 needs collection review (collection)", "1 remaining"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("summary %q does not contain %q", stdout.String(), want)
		}
	}
}
