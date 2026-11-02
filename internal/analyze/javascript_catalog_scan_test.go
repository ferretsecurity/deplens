package analyze

import (
	"path/filepath"
	"testing"
)

func TestJavaScriptCatalogScanResolvesUsedEntries(t *testing.T) {
	result, err := Scan(filepath.Join("..", "..", "testdata", "js", "catalogs"), nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatal(err)
	}
	root := sourceForPath(t, result, "package.json")
	if root.Analysis.Extraction != ExtractionComplete || len(root.Dependencies) != 1 {
		t.Fatalf("root = %+v", root)
	}
	dependency := root.Dependencies[0]
	assertDependency(t, dependency, "react", "^18.3.1", "dependencies", RelationshipDirect, ScopeRuntime)
	if dependency.Raw != "react@catalog:" || dependency.VERS != "vers:npm/>=18.3.1|<19.0.0" || dependency.OriginKind != OriginRegistry ||
		dependency.Attributes["catalog_name"] != "default" || dependency.Attributes["catalog_source"] != "pnpm-workspace.yaml" ||
		dependency.Attributes["catalog_specifier"] != "^18.3.1" {
		t.Fatalf("resolved dependency = %+v", dependency)
	}
	member := sourceForPath(t, result, "packages/app/package.json")
	if member.Analysis.Extraction != ExtractionComplete || len(member.Dependencies) != 3 {
		t.Fatalf("member = %+v", member)
	}
	alias := dependencyByName(t, member.Dependencies, "@acme/server")
	assertDependency(t, alias, "@acme/server", "~3.2.0", "devDependencies", RelationshipDirect, ScopeDevelopment)
	if alias.Raw != "server@catalog:tools" || alias.Attributes["declared_name"] != "server" || alias.VERS == "" {
		t.Fatalf("alias = %+v", alias)
	}
	if local := dependencyByName(t, member.Dependencies, "local"); local.OriginKind != OriginPath || local.Attributes["path"] != "../local" || local.VERS != "" {
		t.Fatalf("local = %+v", local)
	}
	if git := dependencyByName(t, member.Dependencies, "toolkit"); git.OriginKind != OriginGit || git.Attributes["source_ref"] != "v2" || git.VERS != "" {
		t.Fatalf("git = %+v", git)
	}
	config := sourceForPath(t, result, "pnpm-workspace.yaml")
	if len(config.Dependencies) != 0 || config.Analysis != identifiedAnalysis() {
		t.Fatalf("config definitions must not become dependencies: %+v", config)
	}
}

func TestJavaScriptCatalogScanDefaultSemanticsWithoutChecks(t *testing.T) {
	for _, test := range []struct {
		name, configPath, config, specifier, constraint string
	}{
		{"pnpm implicit default", "pnpm-workspace.yaml", "catalog:\n  react: ^18.3.1\n", "catalog:", "^18.3.1"},
		{"pnpm explicit default", "pnpm-workspace.yml", "catalog:\n  react: ^18.3.1\n", "catalog:default", "^18.3.1"},
		{"pnpm named default alias", "pnpm-workspace.yaml", "catalogs:\n  default:\n    react: ^18.3.1\n", "catalog:", "^18.3.1"},
		{"pnpm named", "pnpm-workspace.yaml", "catalogs:\n  legacy:\n    react: ^17.0.0\n", "catalog:legacy", "^17.0.0"},
		{"yarn implicit default", ".yarnrc.yml", "catalog:\n  react: ^18.3.1\ncatalogs:\n  default:\n    react: ^19.0.0\n", "catalog:", "^18.3.1"},
		{"yarn named default", ".yarnrc.yml", "catalog:\n  react: ^18.3.1\ncatalogs:\n  default:\n    react: ^19.0.0\n", "catalog:default", "^19.0.0"},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := scanJavaScriptCatalogFiles(t, map[string]string{
				test.configPath: test.config,
				"package.json": `{"dependencies":{"react":"` + test.specifier + `"}}`,
			})
			source := sourceForPath(t, result, "package.json")
			if source.Analysis.Extraction != ExtractionComplete || len(source.Dependencies) != 1 || source.Dependencies[0].VersionConstraint != test.constraint || source.Dependencies[0].VERS == "" {
				t.Fatalf("source = %+v", source)
			}
			if len(result.CheckRuns) != 0 || len(result.Findings) != 0 {
				t.Fatalf("checks must be disabled: %+v", result)
			}
		})
	}
}

func TestJavaScriptCatalogScanRejectsUnresolvedReferences(t *testing.T) {
	for _, test := range []struct {
		name, config, specifier string
	}{
		{"missing catalog", "packages: []\n", "catalog:"},
		{"missing named catalog", "catalog:\n  react: ^18.3.1\n", "catalog:missing"},
		{"missing entry", "catalog:\n  other: ^1.0.0\n", "catalog:"},
		{"malformed yaml", "catalog: [\n", "catalog:"},
		{"non mapping catalog", "catalog: []\n", "catalog:"},
		{"non string entry", "catalog:\n  react: 18\n", "catalog:"},
		{"non scalar entry", "catalog:\n  react: {version: ^18.3.1}\n", "catalog:"},
		{"empty entry", "catalog:\n  react: ''\n", "catalog:"},
		{"duplicate entry", "catalog:\n  react: ^18.3.1\n  react: ^19.0.0\n", "catalog:"},
		{"duplicate catalog key", "catalog:\n  react: ^18.3.1\ncatalog:\n  react: ^19.0.0\n", "catalog:"},
		{"duplicate named catalog", "catalogs:\n  legacy: {react: ^17.0.0}\n  legacy: {react: ^18.3.1}\n", "catalog:legacy"},
		{"ambiguous default", "catalog: {react: ^18.3.1}\ncatalogs:\n  default: {react: ^19.0.0}\n", "catalog:"},
		{"recursive catalog protocol", "catalog:\n  react: catalog:other\n", "catalog:"},
		{"recursive yaml alias", "catalog: &catalog\n  react: *catalog\n", "catalog:"},
		{"unknown target protocol", "catalog:\n  react: unknown:target\n", "catalog:"},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := scanJavaScriptCatalogFiles(t, map[string]string{
				"pnpm-workspace.yaml": test.config,
				"package.json": `{"dependencies":{"react":"` + test.specifier + `","unrelated":"^1.0.0"}}`,
			})
			source := sourceForPath(t, result, "package.json")
			dependency := dependencyByName(t, source.Dependencies, "react")
			if source.Analysis.Extraction != ExtractionPartial || len(source.Diagnostics) != 1 || source.Diagnostics[0].Code != "javascript-catalog-unresolved" ||
				dependency.Raw != "react@"+test.specifier || dependency.VersionConstraint != "" || dependency.VERS != "" || dependency.OriginKind != "" || dependency.Attributes["catalog_name"] == "" {
				t.Fatalf("unresolved source = %+v", source)
			}
			if dependencyByName(t, source.Dependencies, "unrelated").VERS == "" {
				t.Fatalf("unrelated dependency lost normalization: %+v", source)
			}
		})
	}
}

func TestJavaScriptCatalogScanWorkspaceOwnership(t *testing.T) {
	for _, test := range []struct {
		name, manifestPath string
		files              map[string]string
		constraint         string
	}{
		{"pnpm member", "packages/app/package.json", map[string]string{"pnpm-workspace.yaml": "packages: ['packages/*', '!packages/excluded']\ncatalog: {react: ^18.3.1}\n"}, "^18.3.1"},
		{"pnpm excluded", "packages/excluded/package.json", map[string]string{"pnpm-workspace.yaml": "packages: ['packages/*', '!packages/excluded']\ncatalog: {react: ^18.3.1}\n"}, ""},
		{"unrelated nested package", "examples/app/package.json", map[string]string{"pnpm-workspace.yaml": "packages: ['packages/*']\ncatalog: {react: ^18.3.1}\n"}, ""},
		{"yarn member", "packages/app/package.json", map[string]string{"package.json": `{"workspaces":{"packages":["packages/*"]}}`, ".yarnrc.yml": "catalog: {react: ^18.3.1}\n"}, "^18.3.1"},
		{"yarn unrelated nested package", "examples/app/package.json", map[string]string{"package.json": `{"workspaces":["packages/*"]}`, ".yarnrc.yml": "catalog: {react: ^18.3.1}\n"}, ""},
		{"nested workspace owns itself", "packages/nested/package.json", map[string]string{"pnpm-workspace.yaml": "packages: ['packages/**']\ncatalog: {react: ^18.3.1}\n", "packages/nested/pnpm-workspace.yaml": "packages: ['apps/*']\ncatalog: {react: ^19.0.0}\n"}, "^19.0.0"},
		{"nested workspace member", "packages/nested/apps/app/package.json", map[string]string{"pnpm-workspace.yaml": "packages: ['packages/**']\ncatalog: {react: ^18.3.1}\n", "packages/nested/pnpm-workspace.yaml": "packages: ['apps/*']\ncatalog: {react: ^19.0.0}\n"}, "^19.0.0"},
		{"nested workspace cannot borrow", "packages/nested/apps/app/package.json", map[string]string{"pnpm-workspace.yaml": "packages: ['packages/**']\ncatalog: {react: ^18.3.1}\n", "packages/nested/pnpm-workspace.yaml": "packages: ['apps/*']\n"}, ""},
		{"nested manifest workspace blocks outer", "packages/nested/apps/app/package.json", map[string]string{"pnpm-workspace.yaml": "packages: ['packages/**']\ncatalog: {react: ^18.3.1}\n", "packages/nested/package.json": `{"workspaces":["apps/*"]}`}, ""},
		{"conflicting manager configurations", "package.json", map[string]string{"pnpm-workspace.yaml": "catalog: {react: ^18.3.1}\n", ".yarnrc.yml": "catalog: {react: ^19.0.0}\n"}, ""},
		{"duplicate pnpm configurations", "package.json", map[string]string{"pnpm-workspace.yaml": "catalog: {react: ^18.3.1}\n", "pnpm-workspace.yml": "catalog: {react: ^19.0.0}\n"}, ""},
		{"no configuration", "package.json", map[string]string{}, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			test.files[test.manifestPath] = `{"dependencies":{"react":"catalog:"}}`
			result := scanJavaScriptCatalogFiles(t, test.files)
			source := sourceForPath(t, result, test.manifestPath)
			dependency := dependencyByName(t, source.Dependencies, "react")
			if dependency.VersionConstraint != test.constraint {
				t.Fatalf("dependency = %+v, want constraint %q", dependency, test.constraint)
			}
			if test.constraint == "" && (source.Analysis.Extraction != ExtractionPartial || len(source.Diagnostics) != 1) {
				t.Fatalf("unresolved source = %+v", source)
			}
		})
	}
}

func scanJavaScriptCatalogFiles(t *testing.T, files map[string]string) ScanResult {
	t.Helper()
	root := t.TempDir()
	for path, content := range files {
		mustWriteFile(t, filepath.Join(root, filepath.FromSlash(path)), content)
	}
	ruleset := mustLoadDefaultRules(t)
	ruleset.checks = nil
	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
