package analyze

import (
	"slices"
	"testing"
)

func TestPresetIntegrity(t *testing.T) {
	rules, err := LoadDefaultRules()
	if err != nil {
		t.Fatal(err)
	}
	if len(coveragePresets) != 2 {
		t.Fatal(coveragePresets)
	}
	for name, count := range map[string]int{"snyk": 43, "socket": 41} {
		ids := coveragePresets[name]
		if len(ids) != count {
			t.Fatalf("%s: %d IDs", name, len(ids))
		}
		seen := map[string]bool{}
		for _, id := range ids {
			if seen[id] || !slices.Contains(rules.DetectorIDs(), DetectorID(id)) {
				t.Fatalf("%s invalid member %s", name, id)
			}
			seen[id] = true
		}
	}
}
