package main

import (
	"bytes"
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
	for _, document := range []string{
		"version: 1\ndetectors: []\n",
		"version: 2\nlimits:\n  queries: 8\nunknown: true\ndetectors: []\n",
		"version: 2\nlimits:\n  queries: 8\ndetectors:\n  - id: example\n    state: needs-query-review\n    query_plan: [filename:x]\n",
	} {
		if err := os.WriteFile(path, []byte(document), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := readProgress(path)
		if err == nil {
			t.Fatalf("readProgress accepted %q", document)
		}
		if strings.Contains(document, "version: 1") && !strings.Contains(err.Error(), "initialize a fresh collection") {
			t.Fatalf("old progress error = %v", err)
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

func TestQueryReviewDoesNotBlockCollection(t *testing.T) {
	detectors := []DetectorProgress{{ID: "review", State: stateNeedsQueryReview}, {ID: "ready", State: statePending, QueryPlan: []string{"filename:ready"}}}
	if got := selectDetector(detectors, ""); got == nil || got.ID != "ready" {
		t.Fatalf("selected = %+v", got)
	}
	if got := collectionSummary(Progress{Detectors: detectors}, io.Discard); got != 0 {
		t.Fatalf("summary exit = %d", got)
	}
}
