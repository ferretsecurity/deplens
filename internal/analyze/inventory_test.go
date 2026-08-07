package analyze

import "testing"

func TestDefaultDetectorInventoryClassifiesConfiguredGenericAnalyzers(t *testing.T) {
	inventory, err := DefaultDetectorInventory()
	if err != nil {
		t.Fatalf("DefaultDetectorInventory() error = %v", err)
	}

	for _, id := range []string{"python-pyproject", "python-pipfile", "python-setup-cfg"} {
		entry := inventoryEntry(t, inventory, id)
		if !containsCapability(entry.Capabilities, "extract") {
			t.Fatalf("%s capabilities = %v, want extract", id, entry.Capabilities)
		}
	}
	for _, id := range []string{"python-pdm-lock", "python-conda-environment", "dart-pubspec"} {
		entry := inventoryEntry(t, inventory, id)
		if containsCapability(entry.Capabilities, "extract") {
			t.Fatalf("%s capabilities = %v, do not want extract", id, entry.Capabilities)
		}
	}
}

func inventoryEntry(t *testing.T, inventory []DetectorInventoryEntry, id string) DetectorInventoryEntry {
	t.Helper()
	for _, entry := range inventory {
		if entry.ID == id {
			return entry
		}
	}
	t.Fatalf("detector %q not found", id)
	return DetectorInventoryEntry{}
}

func containsCapability(capabilities []string, want string) bool {
	for _, capability := range capabilities {
		if capability == want {
			return true
		}
	}
	return false
}
