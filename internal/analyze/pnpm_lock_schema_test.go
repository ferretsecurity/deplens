package analyze

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPNPMLockPackageKeySchemas(t *testing.T) {
	for _, tc := range []struct {
		schema, version, react, docsearch string
	}{
		{"v5", "5.4", "18.2.0", "3.3.0"},
		{"v6", "6.0", "18.2.0", "3.5.2"},
		{"v9", "9.0", "18.3.1", "3.7.0"},
	} {
		t.Run(tc.schema, func(t *testing.T) {
			path := filepath.Join("..", "..", "testdata", "javascript", "pnpm-lock-"+tc.schema+"-package-keys", "pnpm-lock.yaml")
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			want := []DependencyReference{
				{Raw: "react@" + tc.react, Name: "react", Version: tc.react, SourceGroup: "dependencies"},
				{Raw: "@docsearch/react@" + tc.docsearch, Name: "@docsearch/react", Version: tc.docsearch},
				{Raw: "loose-envify@1.4.0", Name: "loose-envify", Version: "1.4.0"},
			}
			if tc.schema != "v5" {
				want[0].Attributes = map[string]string{"specifier": "^" + tc.react}
				want = append(want, DependencyReference{Raw: "react-dom@" + tc.react, Name: "react-dom", Version: tc.react})
			}
			for _, version := range []string{tc.version, "'" + tc.version + "'"} {
				t.Run(version, func(t *testing.T) {
					_, body, _ := strings.Cut(string(content), "\n")
					got, err := (pnpmLockParser{}).Analyze(path, []byte("lockfileVersion: "+version+"\n"+body))
					if err != nil {
						t.Fatal(err)
					}
					if !got.Recognized || got.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) || len(got.Diagnostics) != 0 {
						t.Fatalf("unexpected analysis: %+v", got)
					}
					if !equalDependencies(got.Dependencies, want) {
						t.Fatalf("unexpected dependencies: got %+v want %+v", got.Dependencies, want)
					}
				})
			}
		})
	}
}

func TestPNPMLockPackagePeerContextsDeduplicate(t *testing.T) {
	for _, tc := range []struct{ version, prefix string }{{"6.0", "/"}, {"9.0", ""}} {
		t.Run(tc.version, func(t *testing.T) {
			content := "lockfileVersion: '" + tc.version + "'\npackages:\n" +
				"  '" + tc.prefix + "@example/widget@1.2.3(react@18.2.0)': {}\n" +
				"  '" + tc.prefix + "@example/widget@1.2.3(react@18.3.1)': {}\n"
			got, err := (pnpmLockParser{}).Analyze("pnpm-lock.yaml", []byte(content))
			if err != nil {
				t.Fatal(err)
			}
			want := []DependencyReference{{Raw: "@example/widget@1.2.3", Name: "@example/widget", Version: "1.2.3"}}
			if !equalDependencies(got.Dependencies, want) {
				t.Fatalf("unexpected dependencies: got %+v want %+v", got.Dependencies, want)
			}
		})
	}
}
