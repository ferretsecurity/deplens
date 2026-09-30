package analyze

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestPNPMLockPackagesYAMLCompatibility(t *testing.T) {
	var large strings.Builder
	for i := 0; i < 128; i++ {
		fmt.Fprintf(&large, "  package-%03d@1.0.0: {resolution: {integrity: sha512-fixture}}\n", i)
	}
	padding := large.String()
	tests := map[string]string{
		"large unique":       "packages:\n" + padding,
		"large duplicate":    "packages:\n" + padding + "  package-000@1.0.0: {}\n  package-000@1.0.0: {}\n",
		"duplicate key tags": "packages:\n" + padding + "  !!str 1: {}\n  !!int 1: {}\n",
		"scalar keys":        "packages:\n" + padding + "  true: {}\n  1: {}\n  null: {}\n",
		"alias key":          "name: &name package-key\npackages:\n" + padding + "  *name: {}\n",
		"complex key":        "packages:\n" + padding + "  ? [a, b]\n  : {}\n",
		"merge":              "base: &base {react@1.0.0: {}}\npackages:\n" + padding + "  <<: *base\n  react@1.0.0: {explicit: true}\n",
		"merge sequence":     "a: &a {react@1.0.0: {first: true}}\nb: &b {react@1.0.0: {second: true}, vite@2.0.0: {}}\npackages:\n" + padding + "  <<: [*a, *b]\n",
		"alias mapping":      "base: &base\n" + padding + "packages: *base\n",
		"value nodes":        "value: &value {flag: true}\npackages:\n" + padding + "  alias@1.0.0: *value\n  number@1.0.0: 12\n  null@1.0.0: null\n  text@1.0.0: !!str true\n",
		"quoted merge key":   "packages:\n" + padding + "  '<<': {}\n",
		"empty":              "packages: {}\n",
		"null":               "packages: null\n",
		"sequence":           "packages: [a, b]\n",
		"scalar":             "packages: unexpected\n",
		"malformed":          "packages: [\n",
		"unknown alias":      "packages: *missing\n",
		"root duplicates":    "packages:\n" + padding + "packages: {}\n",
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			var want struct {
				Packages map[string]yaml.Node `yaml:"packages"`
			}
			var got struct {
				Packages pnpmLockPackages `yaml:"packages"`
			}
			wantErr := yaml.Unmarshal([]byte(content), &want)
			gotErr := yaml.Unmarshal([]byte(content), &got)
			errorText := func(err error) string {
				if err == nil {
					return ""
				}
				return err.Error()
			}
			if errorText(gotErr) != errorText(wantErr) {
				t.Fatalf("error changed:\ngot %v\nwant %v", gotErr, wantErr)
			}
			if !reflect.DeepEqual(map[string]yaml.Node(got.Packages), want.Packages) {
				t.Fatalf("package nodes changed:\ngot %+v\nwant %+v", got.Packages, want.Packages)
			}
		})
	}
}

func TestPNPMLockAliasesAndMergesExtractDependencies(t *testing.T) {
	content := `lockfileVersion: 9.0
base: &base
  react: {version: 18.3.1, specifier: ^18.3.1}
importers:
  .:
    dependencies:
      <<: *base
      vite: &version 6.0.0
    devDependencies:
      typescript: *version
packages:
  react@18.3.1: {}
  vite@6.0.0: {}
`
	got, err := (pnpmLockParser{}).Analyze("pnpm-lock.yaml", []byte(content))
	if err != nil {
		t.Fatal(err)
	}
	want := []DependencyReference{
		{Raw: "react@18.3.1", Name: "react", Version: "18.3.1", SourceGroup: "dependencies", Attributes: map[string]string{"specifier": "^18.3.1"}},
		{Raw: "vite@6.0.0", Name: "vite", Version: "6.0.0", SourceGroup: "dependencies"},
		{Raw: "typescript@6.0.0", Name: "typescript", Version: "6.0.0", SourceGroup: "devDependencies"},
	}
	if !equalDependencies(got.Dependencies, want) || got.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
		t.Fatalf("unexpected result: %+v", got)
	}
}
