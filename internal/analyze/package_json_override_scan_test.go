package analyze

import (
	"path/filepath"
	"testing"
)

func TestScanPackageJSONOverridesStayInDeclaringManifest(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "package.json"), `{"packageManager":"npm@11","overrides":{"only":"1"}}`)
	mustWriteFile(t, filepath.Join(root, "docs/package.json"), `{"devDependencies":{"vite":"5"},"overrides":{"esbuild":"^0.25.12"}}`)
	mustWriteFile(t, filepath.Join(root, "docs/package-lock.json"), `{"lockfileVersion":3,"packages":{"node_modules/esbuild":{"version":"0.25.12"}}}`)
	result, err := Scan(root, nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatal(err)
	}
	top := sourceForPath(t, result, "package.json")
	if len(top.Dependencies) != 1 || top.Dependencies[0].Name != "only" || top.Dependencies[0].Relationship != RelationshipInconclusive {
		t.Fatalf("override-only manifest: %+v", top)
	}
	if findings := findingsForCheck(result.Findings, "javascript-npm-lockfile-missing"); len(findings) != 0 {
		t.Fatalf("override alone triggered a missing-lockfile finding: %+v", findings)
	}
	docs := sourceForPath(t, result, "docs/package.json")
	if docs.Analysis.Extraction != ExtractionComplete || len(docs.Dependencies) != 2 || dependencyByName(t, docs.Dependencies, "vite").Relationship != RelationshipDirect {
		t.Fatalf("nested manifest: %+v", docs)
	}
	override := dependencyByName(t, docs.Dependencies, "esbuild")
	if override.SourceGroup != "overrides" || override.VersionConstraint != "^0.25.12" || override.Version != "" || override.Relationship != RelationshipInconclusive {
		t.Fatalf("manifest override: %+v", override)
	}
	lock := sourceForPath(t, result, "docs/package-lock.json")
	if dep := dependencyByName(t, lock.Dependencies, "esbuild"); dep.Version != "0.25.12" || dep.VersionConstraint != "" {
		t.Fatalf("lockfile resolution: %+v", dep)
	}
}

func TestScanBunRootCatalogOverridesPreserveMetadata(t *testing.T) {
	result := scanJavaScriptCatalogFiles(t, map[string]string{
		"package.json": `{
			"workspaces":{"packages":["packages/*"],"catalog":{"react":"^19.0.0"},"catalogs":{"tools":{"server":"npm:@acme/server@~3.2.0"}}},
			"overrides":{"react":"catalog:","parent@^1":{"server":"catalog:tools"}}
		}`,
	})
	source := sourceForPath(t, result, "package.json")
	if source.Analysis.Extraction != ExtractionComplete || len(source.Dependencies) != 2 || len(source.Diagnostics) != 0 {
		t.Fatalf("source = %+v", source)
	}
	dependency := dependencyByName(t, source.Dependencies, "react")
	if dependency.Raw != "react@catalog:" || dependency.SourceGroup != "overrides" || dependency.Relationship != RelationshipInconclusive ||
		dependency.VersionConstraint != "^19.0.0" || dependency.VERS == "" || dependency.Attributes["catalog_source"] != "package.json" {
		t.Fatalf("default override = %+v", dependency)
	}
	alias := dependencyByName(t, source.Dependencies, "@acme/server")
	if alias.Raw != "server@catalog:tools" || alias.SourceGroup != "overrides" || alias.Relationship != RelationshipInconclusive ||
		alias.VersionConstraint != "~3.2.0" || alias.VERS == "" || alias.Attributes["declared_name"] != "server" ||
		alias.Attributes["override_path"] != `["parent@^1","server"]` || alias.Attributes["catalog_name"] != "tools" {
		t.Fatalf("named alias override = %+v", alias)
	}
}

func TestScanBunMemberCatalogOverridesCannotResolve(t *testing.T) {
	result := scanJavaScriptCatalogFiles(t, map[string]string{
		"package.json":              `{"workspaces":{"packages":["packages/*"],"catalog":{"react":"^19.0.0"}}}`,
		"packages/app/package.json": `{"packageManager":"bun@1.3.0","overrides":{"react":"catalog:"}}`,
	})
	source := sourceForPath(t, result, "packages/app/package.json")
	if source.Analysis.Extraction != ExtractionPartial || len(source.Dependencies) != 1 || len(source.Diagnostics) != 1 {
		t.Fatalf("source = %+v", source)
	}
	dependency := source.Dependencies[0]
	if dependency.Raw != "react@catalog:" || dependency.OriginKind != "" || dependency.VersionConstraint != "" || dependency.VERS != "" || dependency.Relationship != RelationshipInconclusive {
		t.Fatalf("member override = %+v", dependency)
	}
}
