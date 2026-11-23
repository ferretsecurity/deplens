package analyze

import (
	"strings"
	"testing"
)

func TestBunCatalogScanDefaultsAndPlacements(t *testing.T) {
	for _, test := range []struct{ name, fields, specifier string }{
		{"workspace default", `"workspaces":{"packages":["packages/*"],"catalog":{"react":"^18.3.1"}}`, "catalog:"},
		{"workspace explicit default", `"workspaces":{"catalog":{"react":"^18.3.1"}}`, "catalog:default"},
		{"workspace whitespace default", `"workspaces":{"catalog":{"react":"^18.3.1"}}`, "catalog:  "},
		{"JSON escapes", `"description":"\ud83d\ude00 at https:\/\/example.com","workspaces":[],"catalog":{"react":"^18.3.1"}`, "catalog:"},
		{"empty named default", `"workspaces":[],"catalogs":{"":{"react":"^18.3.1"}}`, "catalog:default"},
		{"top level default", `"workspaces":[],"catalog":{"react":"^18.3.1"}`, "catalog:"},
		{"workspace named", `"workspaces":{"catalogs":{"stable":{"react":"^18.3.1"}}}`, "catalog:stable"},
		{"top level named", `"workspaces":[],"catalogs":{"stable":{"react":"^18.3.1"}}`, "catalog:stable"},
		{"nested wins", `"catalog":{"react":"^19.0.0"},"workspaces":{"catalog":{"react":"^18.3.1"}}`, "catalog:"},
		{"merged top-level default", `"workspaces":[],"catalog":{"unused":"^1.0.0"},"catalogs":{"default":{"react":"^18.3.1"}}`, "catalog:"},
		{"merged local default", `"workspaces":{"catalog":{"unused":"^1.0.0"},"catalogs":{"default":{"react":"^18.3.1"}}}`, "catalog:default"},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := scanJavaScriptCatalogFiles(t, map[string]string{"package.json": `{` + test.fields + `,"dependencies":{"react":"` + test.specifier + `"}}`})
			source := sourceForPath(t, result, "package.json")
			dep := dependencyByName(t, source.Dependencies, "react")
			if source.Analysis.Extraction != ExtractionComplete || len(source.Dependencies) != 1 || dep.VersionConstraint != "^18.3.1" || dep.VERS != "vers:npm/>=18.3.1|<19.0.0" || dep.Attributes["catalog_source"] != "package.json" || dep.Attributes["catalog_specifier"] != "^18.3.1" || dep.Raw != "react@"+test.specifier {
				t.Fatalf("source = %+v", source)
			}
		})
	}
}

func TestBunCatalogScanAliasAndOwnership(t *testing.T) {
	for _, test := range []struct {
		name, member string
		files        map[string]string
		resolved     bool
	}{
		{"member", "packages/app/package.json", nil, true},
		{"member ignores local catalog without workspaces", "packages/app/package.json", map[string]string{"packages/app/package.json": `{"catalog":{"alias":"^1.0.0"},"dependencies":{"alias":"catalog:"}}`}, true},
		{"nonmember", "examples/app/package.json", nil, false},
		{"excluded member", "packages/excluded/package.json", nil, false},
		{"nested empty workspace", "packages/nested/apps/app/package.json", map[string]string{"packages/nested/package.json": `{"workspaces":[]}`}, false},
		{"nested workspace", "packages/nested/apps/app/package.json", map[string]string{"packages/nested/package.json": `{"workspaces":["apps/*"]}`}, false},
		{"nested independent manager", "packages/nested/apps/app/package.json", map[string]string{"packages/nested/package.json": `{"packageManager":"npm@10"}`}, false},
		{"member manager conflicts", "packages/app/package.json", map[string]string{"packages/app/package.json": `{"packageManager":"pnpm@10","dependencies":{"alias":"catalog:"}}`}, false},
		{"root manager conflicts", "packages/app/package.json", map[string]string{"package.json": `{"packageManager":"pnpm@10","workspaces":{"packages":["packages/**"],"catalog":{"alias":"npm:@acme/actual@^2.0.0"}}}`}, false},
		{"manager config conflicts", "packages/app/package.json", map[string]string{"pnpm-workspace.yaml": "packages: ['packages/**']\ncatalog: {alias: '^1.0.0'}\n"}, false},
		{"bun manager", "packages/app/package.json", map[string]string{"package.json": `{"packageManager":"bun@1.3.0","workspaces":{"packages":["packages/**"],"catalog":{"alias":"npm:@acme/actual@^2.0.0"}}}`}, true},
		{"nested own catalog", "packages/nested/apps/app/package.json", map[string]string{"packages/nested/package.json": `{"workspaces":{"packages":["apps/*"],"catalog":{"alias":"npm:@acme/actual@^2.0.0"}}}`}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			files := map[string]string{"package.json": `{"workspaces":{"packages":["packages/**","!packages/excluded"],"catalog":{"alias":"npm:@acme/actual@^2.0.0"}}}`, test.member: `{"dependencies":{"alias":"catalog:"}}`}
			for path, content := range test.files {
				files[path] = content
			}
			result := scanJavaScriptCatalogFiles(t, files)
			source := sourceForPath(t, result, test.member)
			if test.resolved {
				dep := dependencyByName(t, source.Dependencies, "@acme/actual")
				if source.Analysis.Extraction != ExtractionComplete || dep.VersionConstraint != "^2.0.0" || dep.Attributes["declared_name"] != "alias" || dep.Raw != "alias@catalog:" {
					t.Fatalf("source = %+v", source)
				}
			} else if source.Analysis.Extraction != ExtractionPartial || dependencyByName(t, source.Dependencies, "alias").OriginKind != "" || len(source.Diagnostics) != 1 {
				t.Fatalf("source = %+v", source)
			}
		})
	}
}

func TestBunCatalogScanRejectsMalformedDefinitionsAndTargets(t *testing.T) {
	for _, fields := range []string{
		`"catalog":{"react":"^18.3.1","react":"^19.0.0"}`,
		`"catalog":{"react":"^18.3.1"},"catalog":{"react":"^19.0.0"}`,
		`"catalog":{"react":"^18.3.1"},"catalogs":{"default":{"react":"^19.0.0"}}`,
		`"workspaces":null,"catalog":{"react":"^18.3.1"}`, `"workspaces":"invalid","catalog":{"react":"^18.3.1"}`,
		`"workspaces":{"packages":[null],"catalog":{"react":"^18.3.1"}}`,
		`"catalog":[]`, `"catalog":{"react":18}`, `"catalog":{"react":null}`, `"catalog":{"react":""}`,
		`"catalog":{"react":"catalog:other"}`, `"catalog":{"react":"unknown:target"}`,
		`"catalog":{"react":"npm:"}`, `"catalog":{"react":"npm:@broken"}`, `"catalog":{"react":"npm:valid@bad range"}`,
		`"catalog":{"react":"bad range"}`, `"workspaces":{"catalogs":[]}`,
	} {
		t.Run(fields, func(t *testing.T) {
			if !strings.Contains(fields, `"workspaces"`) {
				fields = `"workspaces":[],` + fields
			}
			result := scanJavaScriptCatalogFiles(t, map[string]string{"package.json": `{` + fields + `,"dependencies":{"react":"catalog:","unrelated":"^1.0.0"}}`})
			source := sourceForPath(t, result, "package.json")
			dep := dependencyByName(t, source.Dependencies, "react")
			if source.Analysis.Extraction != ExtractionPartial || len(source.Diagnostics) != 1 || source.Diagnostics[0].Code != "javascript-catalog-unresolved" || dep.OriginKind != "" || dep.VERS != "" || dep.Raw != "react@catalog:" || dependencyByName(t, source.Dependencies, "unrelated").VERS == "" {
				t.Fatalf("source = %+v", source)
			}
		})
	}
}

func TestBunCatalogScanMissingAndIgnoredDefinitions(t *testing.T) {
	for _, test := range []struct{ name, fields, specifier string }{
		{"top-level requires workspaces", `"catalog":{"react":"^18.3.1"}`, "catalog:"},
		{"missing catalog", `"workspaces":[],"catalogs":{}`, "catalog:"},
		{"missing entry", `"workspaces":[],"catalog":{"other":"^1.0.0"}`, "catalog:"},
		{"missing named catalog", `"workspaces":[],"catalog":{"react":"^18.3.1"}`, "catalog:missing"},
		{"nested singular ignores top-level named", `"catalogs":{"stable":{"react":"^18.3.1"}},"workspaces":{"catalog":{"other":"^1.0.0"}}`, "catalog:stable"},
		{"nested plural ignores top-level singular", `"catalog":{"react":"^18.3.1"},"workspaces":{"catalogs":{"stable":{"react":"^19.0.0"}}}`, "catalog:"},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := scanJavaScriptCatalogFiles(t, map[string]string{"package.json": `{` + test.fields + `,"dependencies":{"react":"` + test.specifier + `"}}`})
			source := sourceForPath(t, result, "package.json")
			dep := dependencyByName(t, source.Dependencies, "react")
			if source.Analysis.Extraction != ExtractionPartial || len(source.Diagnostics) != 1 || source.Diagnostics[0].Code != "javascript-catalog-unresolved" || dep.OriginKind != "" || dep.VERS != "" {
				t.Fatalf("source = %+v", source)
			}
		})
	}
}
