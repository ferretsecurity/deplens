package analyze

import (
	"strings"
	"testing"
)

func TestDefaultRulesUseCompleteDependencySourceMetadata(t *testing.T) {
	ruleset, err := LoadDefaultRules()
	if err != nil {
		t.Fatalf("LoadDefaultRules failed: %v", err)
	}
	if len(ruleset.detectors) != 185 {
		t.Fatalf("expected 185 built-in detectors, got %d", len(ruleset.detectors))
	}
	for index, detector := range ruleset.detectors {
		if detector.ID == "" || !validSourceForm(detector.Form) || len(detector.Roles) == 0 {
			t.Fatalf("detector %d has incomplete metadata: %+v", index, detector)
		}
	}
}

func TestRuleSchemaRejectsLegacyTopLevelFields(t *testing.T) {
	_, err := loadRules("legacy.yaml", []byte(`
rules:
  - name: js
    form: manifest
    roles: [declaration]
    filename-regex: '^package\.json$'
`))
	if err == nil || !strings.Contains(err.Error(), "field name not found") {
		t.Fatalf("expected strict rejection of legacy name field, got %v", err)
	}
}

func TestRuleSchemaRejectsLegacyAnalyzerPlacement(t *testing.T) {
	_, err := loadRules("legacy.yaml", []byte(`
rules:
  - id: js
    form: manifest
    roles: [declaration]
    filename-regex: '^package\.json$'
    json:
      exists-any: [dependencies]
`))
	if err == nil || !strings.Contains(err.Error(), "field json not found") {
		t.Fatalf("expected strict rejection of top-level analyzer field, got %v", err)
	}
}

func TestRuleSchemaRejectsLegacyDependencyTypeField(t *testing.T) {
	_, err := loadRules("legacy.yaml", []byte(`
rules:
  - id: js
    dependency-type: npm
    form: manifest
    roles: [declaration]
    filename-regex: '^package\.json$'
`))
	if err == nil || !strings.Contains(err.Error(), "field dependency-type not found") {
		t.Fatalf("expected strict rejection of legacy dependency-type field, got %v", err)
	}
}

func TestRuleSchemaRejectsUnknownAnalyzerConfiguration(t *testing.T) {
	_, err := loadRules("invalid.yaml", []byte(`
rules:
  - id: js
    form: manifest
    roles: [declaration]
    filename-regex: '^package\.json$'
    analyzer:
      type: json
      exists-any: [dependencies]
      typo: true
`))
	if err == nil || !strings.Contains(err.Error(), "field typo not found") {
		t.Fatalf("expected strict rejection of unknown analyzer field, got %v", err)
	}
}

func TestRuleSchemaRejectsDuplicateDetectorIDs(t *testing.T) {
	_, err := loadRules("invalid.yaml", []byte(`
rules:
  - id: duplicate
    form: manifest
    roles: [declaration]
    filename-regex: '^one$'
  - id: duplicate
    form: lockfile
    roles: [resolution]
    filename-regex: '^two$'
`))
	if err == nil || !strings.Contains(err.Error(), "duplicate value") {
		t.Fatalf("expected duplicate detector ID error, got %v", err)
	}
}

func TestRuleSchemaRejectsInvalidFormAndRoles(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{name: "invalid form", yaml: "rules:\n  - id: source\n    form: file\n    roles: [declaration]\n    filename-regex: '^source$'\n"},
		{name: "missing roles", yaml: "rules:\n  - id: source\n    form: manifest\n    filename-regex: '^source$'\n"},
		{name: "duplicate roles", yaml: "rules:\n  - id: source\n    form: manifest\n    roles: [declaration, declaration]\n    filename-regex: '^source$'\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := loadRules("invalid.yaml", []byte(test.yaml)); err == nil {
				t.Fatalf("expected invalid rule metadata to be rejected")
			}
		})
	}
}

func TestAnalyzerResultValidationRejectsImpossibleStates(t *testing.T) {
	tests := []sourceAnalyzerResult{
		{Recognized: true, Analysis: SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}, Dependencies: []DependencyReference{{Raw: "unexpected"}}},
		{Recognized: true, Analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionPartial}, Dependencies: []DependencyReference{{Raw: "partial"}}},
		{Recognized: true, Analysis: SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionFailed}},
	}
	for _, result := range tests {
		if err := validateSourceAnalyzerResult(result); err == nil {
			t.Fatalf("expected invalid analyzer result to be rejected: %+v", result)
		}
	}
}
