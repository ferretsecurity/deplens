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
