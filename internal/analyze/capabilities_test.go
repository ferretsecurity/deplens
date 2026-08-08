package analyze

import "testing"

func TestDefaultDetectorCapabilitiesMatchCollectionEligibility(t *testing.T) {
	rules, err := LoadDefaultRules()
	if err != nil {
		t.Fatalf("load default rules: %v", err)
	}
	eligible := 0
	for _, capability := range rules.DetectorCapabilities() {
		if !containsCapability(capability.Capabilities, "extract") {
			eligible++
		}
	}
	if eligible != 144 {
		t.Fatalf("expected 144 non-extracting detectors, got %d", eligible)
	}
}

func TestDetectorInventoryFingerprintIncludesAnalyzerConfiguration(t *testing.T) {
	first, err := loadRules("first", []byte(`rules:
  - id: yaml-rule
    package-type: npm
    form: manifest
    roles: [declaration]
    filename-regex: '^example\\.yaml$'
    analyzer:
      type: yaml
      query: dependencies
`))
	if err != nil {
		t.Fatalf("load first rules: %v", err)
	}
	second, err := loadRules("second", []byte(`rules:
  - id: yaml-rule
    package-type: npm
    form: manifest
    roles: [declaration]
    filename-regex: '^example\\.yaml$'
    analyzer:
      type: yaml
      query: devDependencies
`))
	if err != nil {
		t.Fatalf("load second rules: %v", err)
	}
	if first.DetectorInventoryFingerprint() == second.DetectorInventoryFingerprint() {
		t.Fatal("expected analyzer configuration to change the inventory fingerprint")
	}
}

func containsCapability(capabilities []string, wanted string) bool {
	for _, capability := range capabilities {
		if capability == wanted {
			return true
		}
	}
	return false
}
