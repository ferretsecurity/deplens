package analyze

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDefaultRulesScanPackageLockAliases(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	fixture, err := os.ReadFile(filepath.Join("..", "..", "testdata", "javascript", "package-lock-v3-aliases", "package-lock.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := []DependencyReference{
		{PackageType: "npm", Raw: "@local/scoped-alias@2.0.0", Name: "@acme/scoped", Version: "2.0.0", SourceGroup: "optionalDependencies", Attributes: map[string]string{"declared_name": "@local/scoped-alias"}},
		{PackageType: "npm", Raw: "@types/node@20.12.7", Name: "@types/node", Version: "20.12.7", SourceGroup: "devDependencies"},
		{PackageType: "npm", Raw: "left-pad@1.3.0", Name: "left-pad", Version: "1.3.0", SourceGroup: "dependencies"},
		{PackageType: "npm", Raw: "parent@1.0.0", Name: "parent", Version: "1.0.0", SourceGroup: "dependencies"},
		{PackageType: "npm", Raw: "nested-color@3.0.3", Name: "kleur", Version: "3.0.3", Attributes: map[string]string{"declared_name": "nested-color"}},
		{PackageType: "npm", Raw: "string-width@5.1.2", Name: "string-width", Version: "5.1.2", SourceGroup: "devDependencies"},
		{PackageType: "npm", Raw: "string-width-cjs@4.2.3", Name: "string-width", Version: "4.2.3", SourceGroup: "dependencies", Attributes: map[string]string{"declared_name": "string-width-cjs"}},
	}

	for _, version := range []int{2, 3} {
		for _, tc := range []struct {
			filename string
			detector DetectorID
		}{
			{filename: "package-lock.json", detector: "js-npm-lock"},
			{filename: "npm-shrinkwrap.json", detector: "js-npm-shrinkwrap"},
		} {
			t.Run(fmt.Sprintf("v%d/%s", version, tc.filename), func(t *testing.T) {
				root := t.TempDir()
				content := strings.Replace(string(fixture), `"lockfileVersion": 3`, fmt.Sprintf(`"lockfileVersion": %d`, version), 1)
				mustWriteFile(t, filepath.Join(root, tc.filename), content)

				result, err := Scan(root, nil, ruleset)
				if err != nil {
					t.Fatalf("Scan failed: %v", err)
				}
				if len(result.Sources) != 1 {
					t.Fatalf("expected one dependency source, got %+v", result.Sources)
				}
				source := result.Sources[0]
				if source.Path != tc.filename || source.Detector != tc.detector {
					t.Fatalf("unexpected dependency source: %+v", source)
				}
				if source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
					t.Fatalf("unexpected analysis: %+v", source.Analysis)
				}
				if len(source.Diagnostics) != 0 {
					t.Fatalf("unexpected diagnostics: %+v", source.Diagnostics)
				}
				if !reflect.DeepEqual(source.Dependencies, want) {
					t.Fatalf("dependencies mismatch:\n got: %+v\nwant: %+v", source.Dependencies, want)
				}
			})
		}
	}
}
