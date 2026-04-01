package analyze

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestDetectManifestMatchesSupportedFiles(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	testCases := []struct {
		name string
		want ManifestType
	}{
		{name: "requirements.txt", want: ManifestType("python-requirements")},
		{name: "requirements.in", want: ManifestType("python-requirements")},
		{name: "my-requirements.txt", want: ManifestType("python-requirements")},
		{name: "my-requirements.in", want: ManifestType("python-requirements")},
		{name: "requirements-dev.txt", want: ManifestType("python-requirements")},
		{name: "requirements.dev.in", want: ManifestType("python-requirements")},
		{name: "requirements.qt6_3.in", want: ManifestType("python-requirements")},
		{name: "my_requirements.prod.txt", want: ManifestType("python-requirements")},
		{name: "uv.lock", want: ManifestType("python-uv")},
		{name: "package.json", want: ManifestType("js")},
		{name: "yarn.lock", want: ManifestType("js-yarn")},
		{name: "pom.xml", want: ManifestType("java")},
	}

	for _, tc := range testCases {
		got, ok := ruleset.DetectManifest(tc.name)
		if !ok {
			t.Fatalf("expected %s to be detected", tc.name)
		}
		if got != tc.want {
			t.Fatalf("expected type %q, got %q", tc.want, got)
		}
	}
}

func TestDetectManifestIgnoresSimilarNames(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	testCases := []string{
		"myrequirements.txt",
		"myrequirements.in",
		"requirementsdev.txt",
		"requirementsin",
		"requirements.txt.backup",
		"requirements.in.backup",
		"main.tf",
		"package-lock.json",
		"pom.xml.backup",
		"yarn.lock.old",
		"uv.lock.json",
		"index.html.bak",
		"component.jsx",
	}

	for _, tc := range testCases {
		if _, ok := ruleset.DetectManifest(tc); ok {
			t.Fatalf("expected %s to be ignored", tc)
		}
	}
}

func TestScanFindsNestedManifestsSortedByRelativePath(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "b", "requirements.dev.in"), "")
	mustWriteFile(t, filepath.Join(root, "a", "package.json"), "")
	mustWriteFile(t, filepath.Join(root, "a", "index.html"), `<script src="https://cdn.example.com/app.js"></script>`)
	mustWriteFile(t, filepath.Join(root, "c", "job.tf"), `
resource "aws_glue_job" "python_shell_example" {
  default_arguments = {
    "--job-language" = "python"
    "--additional-python-modules" = "scikit-learn==1.4.1.post1,pandas==2.2.1"
  }
}
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Manifests) != 4 {
		t.Fatalf("expected 4 manifests, got %d", len(result.Manifests))
	}
	if result.Manifests[0].Path != "a/index.html" || result.Manifests[1].Path != "a/package.json" || result.Manifests[2].Path != "b/requirements.dev.in" || result.Manifests[3].Path != "c/job.tf" {
		t.Fatalf("unexpected manifest order: %+v", result.Manifests)
	}
}

func TestScanFindsRequirementsInFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "sample-monorepo"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	for _, manifest := range result.Manifests {
		if manifest.Type == ManifestType("python-requirements") && manifest.Path == "requirements.qt6_3.in" {
			return
		}
	}

	t.Fatalf("expected requirements.qt6_3.in fixture to be detected, got %+v", result.Manifests)
}

func TestScanMatchesHTMLExternalScripts(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "templates", "index.html"), `
<!DOCTYPE html>
<html>
  <head>
    <script src="https://cdn.jsdelivr.net/npm/dompurify@3.0.8/dist/purify.min.js"></script>
    <script src="/assets/app.js"></script>
    <script>console.log("inline")</script>
    <script SRC="https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js"></script>
  </head>
</html>
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	manifest := result.Manifests[0]
	if manifest.Type != ManifestType("html-external-scripts") || manifest.Path != "templates/index.html" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	want := []string{
		"https://cdn.jsdelivr.net/npm/dompurify@3.0.8/dist/purify.min.js",
		"https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js",
	}
	if !slices.Equal(manifest.Dependencies, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", manifest.Dependencies, want)
	}
}

func TestScanMatchesHTMLModuleImportFromTestdata(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "html", "module-import"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}

	manifest := result.Manifests[0]
	if manifest.Type != ManifestType("html-external-scripts") || manifest.Path != "index.html" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}

	want := []string{"https://cdn.jsdelivr.net/npm/swiper@12.1.2/+esm"}
	if !slices.Equal(manifest.Dependencies, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", manifest.Dependencies, want)
	}
}

func TestScanMatchesHTMLNamespaceModuleImportFromTestdata(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "html", "module-namespace-import"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}

	manifest := result.Manifests[0]
	if manifest.Type != ManifestType("html-external-scripts") || manifest.Path != "index.html" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}

	want := []string{"https://cdn.jsdelivr.net/npm/d3@7/+esm"}
	if !slices.Equal(manifest.Dependencies, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", manifest.Dependencies, want)
	}
}

func TestScanMatchesHTMLImportMapFromTestdata(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "html", "importmap"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}

	manifest := result.Manifests[0]
	if manifest.Type != ManifestType("html-external-scripts") || manifest.Path != "index.html" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}

	want := []string{
		"https://cdn.jsdelivr.net/npm/super-media-element@1.3/+esm",
		"https://cdn.jsdelivr.net/npm/media-tracks@0.2/+esm",
		"https://cdn.jsdelivr.net/npm/hls.js@1.6.0-beta.1/dist/hls.mjs",
	}
	if !slices.Equal(manifest.Dependencies, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", manifest.Dependencies, want)
	}
}

func TestScanMatchesHTMLImportMapRemoteImports(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "templates", "index.html"), `
<!DOCTYPE html>
<html>
  <head>
    <script type="importmap">
      {
        "imports": {
          "super-media-element": "https://cdn.jsdelivr.net/npm/super-media-element@1.3/+esm",
          "media-tracks": "https://cdn.jsdelivr.net/npm/media-tracks@0.2/+esm",
          "@superstreamer/player": "/packages/player/dist/index.js",
          "hls.js": "https://cdn.jsdelivr.net/npm/hls.js@1.6.0-beta.1/dist/hls.mjs",
          "stylelint-config-recess-order": "https://registry.npmmirror.com/stylelint-config-recess-order/5.0.0/files/groups.js"
        }
      }
    </script>
  </head>
</html>
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}

	manifest := result.Manifests[0]
	want := []string{
		"https://cdn.jsdelivr.net/npm/super-media-element@1.3/+esm",
		"https://cdn.jsdelivr.net/npm/media-tracks@0.2/+esm",
		"https://cdn.jsdelivr.net/npm/hls.js@1.6.0-beta.1/dist/hls.mjs",
		"https://registry.npmmirror.com/stylelint-config-recess-order/5.0.0/files/groups.js",
	}
	if !slices.Equal(manifest.Dependencies, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", manifest.Dependencies, want)
	}
}

func TestScanMatchesHTMLImportMapESMShImports(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "index.html"), `
<script type="importmap">
{
  "imports": {
    "react/": "https://esm.sh/react@^19.1.0/",
    "react": "https://esm.sh/react@^19.1.0",
    "@google/genai": "https://esm.sh/@google/genai@^1.0.0",
    "recharts": "https://esm.sh/recharts@^2.15.3",
    "react-dom/": "https://esm.sh/react-dom@^19.1.0/"
  }
}
</script>
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}

	want := []string{
		"https://esm.sh/react@^19.1.0/",
		"https://esm.sh/react@^19.1.0",
		"https://esm.sh/@google/genai@^1.0.0",
		"https://esm.sh/recharts@^2.15.3",
		"https://esm.sh/react-dom@^19.1.0/",
	}
	if !slices.Equal(result.Manifests[0].Dependencies, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Manifests[0].Dependencies, want)
	}
}

func TestScanDoesNotMatchHTMLWithoutExternalScripts(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "page.html"), `
<script src="/assets/app.js"></script>
<script src="//cdn.example.com/app.js"></script>
<script>console.log("inline")</script>
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 0 {
		t.Fatalf("expected no manifests, got %+v", result.Manifests)
	}
}

func TestScanMatchesJavaScriptBannerDetectors(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	testCases := []struct {
		path     string
		wantType ManifestType
		wantDeps []string
	}{
		{
			path:     "jquery.min.js",
			wantType: ManifestType("js-banner-block-start"),
			wantDeps: []string{"jQuery@3.7.1"},
		},
		{
			path:     "purify.min.js",
			wantType: ManifestType("js-banner-plain-block-start"),
			wantDeps: []string{"DOMPurify@3.0.8"},
		},
		{
			path:     "bootstrap.min.js",
			wantType: ManifestType("js-banner-multiline-preserved"),
			wantDeps: []string{"Bootstrap@5.3.3"},
		},
		{
			path:     "mustache.min.js",
			wantType: ManifestType("js-banner-line-comment"),
			wantDeps: []string{"Mustache.js@4.2.0"},
		},
		{
			path:     "htmx.min.js",
			wantType: ManifestType("js-banner-version-tagged"),
			wantDeps: []string{"htmx.org@2.0.4"},
		},
	}

	result, err := Scan(filepath.Join("..", "..", "testdata", "js", "banners"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Manifests) != len(testCases) {
		t.Fatalf("expected %d manifests, got %d", len(testCases), len(result.Manifests))
	}

	gotByPath := make(map[string]ManifestMatch, len(result.Manifests))
	for _, manifest := range result.Manifests {
		gotByPath[manifest.Path] = manifest
	}

	for _, tc := range testCases {
		manifest, ok := gotByPath[tc.path]
		if !ok {
			t.Fatalf("expected manifest for %q, got %+v", tc.path, result.Manifests)
		}
		if manifest.Type != tc.wantType {
			t.Fatalf("unexpected manifest type for %q: got %q want %q", tc.path, manifest.Type, tc.wantType)
		}
		if !slices.Equal(manifest.Dependencies, tc.wantDeps) {
			t.Fatalf("unexpected dependencies for %q: got %+v want %+v", tc.path, manifest.Dependencies, tc.wantDeps)
		}
	}
}

func TestScanDoesNotMatchJavaScriptWithoutBanner(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "app.js"), "console.log('no banner')\n")

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 0 {
		t.Fatalf("expected no manifests, got %+v", result.Manifests)
	}
}

func TestScanDoesNotMatchCSSBannerWithDefaultRules(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "style.css"), "/*! Bootstrap v5.3.3 */\n")

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 0 {
		t.Fatalf("expected no manifests, got %+v", result.Manifests)
	}
}

func TestScanMatchesTerraformGluePythonDependencies(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "glue", "job.tf"), `
resource "aws_glue_job" "python_shell_example" {
  default_arguments = {
    "--job-language" = "python"
    "--additional-python-modules" = "scikit-learn==1.4.1.post1,pandas==2.2.1"
  }
}
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if result.Manifests[0].Type != ManifestType("terraform.aws_glue_job.python") || result.Manifests[0].Path != "glue/job.tf" {
		t.Fatalf("unexpected manifest: %+v", result.Manifests[0])
	}
	if len(result.Manifests[0].Dependencies) != 0 {
		t.Fatalf("expected terraform detector to keep dependencies empty, got %+v", result.Manifests[0].Dependencies)
	}
}

func TestScanDoesNotMatchTerraformWithoutAdditionalModules(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "job.tf"), `
resource "aws_glue_job" "python_shell_example" {
  default_arguments = {
    "--job-language" = "python"
  }
}
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 0 {
		t.Fatalf("expected no manifests, got %+v", result.Manifests)
	}
}

func TestScanDoesNotMatchTerraformWithoutPythonLanguage(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "job.tf"), `
resource "aws_glue_job" "scala_job" {
  default_arguments = {
    "--job-language" = "scala"
    "--additional-python-modules" = "pandas==2.2.1"
  }
}
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 0 {
		t.Fatalf("expected no manifests, got %+v", result.Manifests)
	}
}

func TestScanDoesNotMatchNonGlueTerraformResource(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "main.tf"), `
resource "aws_s3_bucket" "example" {
  bucket = "example"
}

locals {
  default_arguments = {
    "--job-language" = "python"
    "--additional-python-modules" = "pandas==2.2.1"
  }
}
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 0 {
		t.Fatalf("expected no manifests, got %+v", result.Manifests)
	}
}

func TestScanMatchesTypeScriptGluePythonDependenciesWithNamespaceImport(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "glue", "job.ts"), `
import * as glue from "aws-cdk-lib/aws-glue";

new glue.CfnJob(this, "Job", {
  defaultArguments: {
    "--job-language": "python",
    "--additional-python-modules": "pandas==2.2.1, scikit-learn==1.4.1.post1,",
  },
});
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if result.Manifests[0].Type != ManifestType("typescript.cdk.aws_glue_job.python") || result.Manifests[0].Path != "glue/job.ts" {
		t.Fatalf("unexpected manifest: %+v", result.Manifests[0])
	}
	wantDependencies := []string{"pandas==2.2.1", "scikit-learn==1.4.1.post1"}
	if !slices.Equal(result.Manifests[0].Dependencies, wantDependencies) {
		t.Fatalf("unexpected dependencies: got %v want %v", result.Manifests[0].Dependencies, wantDependencies)
	}
}

func TestScanMatchesTypeScriptGluePythonDependenciesWithNamedImport(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "job.ts"), `
import { CfnJob as GlueJob } from "aws-cdk-lib/aws-glue";

new GlueJob(this, "Job", {
  defaultArguments: {
    "--job-language": "python",
    "--additional-python-modules": "pandas==2.2.1",
  },
});
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if got := result.Manifests[0].Dependencies; !slices.Equal(got, []string{"pandas==2.2.1"}) {
		t.Fatalf("unexpected dependencies: %+v", got)
	}
}

func TestScanDoesNotMatchTypeScriptWithoutAdditionalModules(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "job.ts"), `
import * as glue from "aws-cdk-lib/aws-glue";

new glue.CfnJob(this, "Job", {
  defaultArguments: {
    "--job-language": "python",
  },
});
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 0 {
		t.Fatalf("expected no manifests, got %+v", result.Manifests)
	}
}

func TestScanDoesNotMatchTypeScriptWithoutPythonLanguage(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "job.ts"), `
import { CfnJob } from "aws-cdk-lib/aws-glue";

new CfnJob(this, "Job", {
  defaultArguments: {
    "--job-language": "scala",
    "--additional-python-modules": "pandas==2.2.1",
  },
});
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 0 {
		t.Fatalf("expected no manifests, got %+v", result.Manifests)
	}
}

func TestScanDoesNotMatchTypeScriptWithUnrelatedImport(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "job.ts"), `
import { CfnJob } from "not/aws-glue";

new CfnJob(this, "Job", {
  defaultArguments: {
    "--job-language": "python",
    "--additional-python-modules": "pandas==2.2.1",
  },
});
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 0 {
		t.Fatalf("expected no manifests, got %+v", result.Manifests)
	}
}

func TestScanMatchesTypeScriptWithVariableProps(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "job.ts"), `
import { CfnJob } from "aws-cdk-lib/aws-glue";

const props = {
  defaultArguments: {
    "--job-language": "python",
    "--additional-python-modules": "pandas==2.2.1",
  },
};

new CfnJob(this, "Job", props);
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if got := result.Manifests[0].Dependencies; !slices.Equal(got, []string{"pandas==2.2.1"}) {
		t.Fatalf("unexpected dependencies: %+v", got)
	}
}

func TestScanMatchesTypeScriptWithVariableAdditionalModules(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "job.ts"), `
import { CfnJob } from "aws-cdk-lib/aws-glue";

const modules = "pandas==2.2.1";

new CfnJob(this, "Job", {
  defaultArguments: {
    "--job-language": "python",
    "--additional-python-modules": modules,
  },
});
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if got := result.Manifests[0].Dependencies; !slices.Equal(got, []string{"pandas==2.2.1"}) {
		t.Fatalf("unexpected dependencies: %+v", got)
	}
}

func TestScanMatchesTypeScriptFixtureFromTestdata(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := filepath.Join("..", "..", "testdata", "typescript", "glue-cfnjob-inline")

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if result.Manifests[0].Type != ManifestType("typescript.cdk.aws_glue_job.python") || result.Manifests[0].Path != "job.ts" {
		t.Fatalf("unexpected manifest: %+v", result.Manifests[0])
	}
	wantDependencies := []string{"pandas==2.2.1", "scikit-learn==1.4.1.post1"}
	if !slices.Equal(result.Manifests[0].Dependencies, wantDependencies) {
		t.Fatalf("unexpected dependencies: got %v want %v", result.Manifests[0].Dependencies, wantDependencies)
	}
}

func TestScanDoesNotMatchTypeScriptNegativeFixturesFromTestdata(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	fixtures := []string{
		filepath.Join("..", "..", "testdata", "typescript", "glue-cfnjob-no-modules"),
	}

	for _, root := range fixtures {
		result, err := Scan(root, nil, ruleset)
		if err != nil {
			t.Fatalf("scan failed for %s: %v", root, err)
		}
		if len(result.Manifests) != 0 {
			t.Fatalf("expected no manifests for %s, got %+v", root, result.Manifests)
		}
	}
}

func TestScanMatchesTypeScriptVariablePropsFixtureFromTestdata(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := filepath.Join("..", "..", "testdata", "typescript", "glue-cfnjob-variable-props")

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if result.Manifests[0].Type != ManifestType("typescript.cdk.aws_glue_job.python") || result.Manifests[0].Path != "job.ts" {
		t.Fatalf("unexpected manifest: %+v", result.Manifests[0])
	}
	if got := result.Manifests[0].Dependencies; !slices.Equal(got, []string{"pandas==2.2.1"}) {
		t.Fatalf("unexpected dependencies: %+v", got)
	}
}

func TestScanMatchesTypeScriptComputedModulesFixtureFromTestdata(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := filepath.Join("..", "..", "testdata", "typescript", "glue-cfnjob-computed-modules")

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if result.Manifests[0].Type != ManifestType("typescript.cdk.aws_glue_job.python") || result.Manifests[0].Path != "job.ts" {
		t.Fatalf("unexpected manifest: %+v", result.Manifests[0])
	}
	if got := result.Manifests[0].Dependencies; !slices.Equal(got, []string{"pandas==2.2.1"}) {
		t.Fatalf("unexpected dependencies: %+v", got)
	}
}

func TestScanMatchesTypeScriptFunctionComputedModulesFixtureFromTestdata(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := filepath.Join("..", "..", "testdata", "typescript", "glue-cfnjob-function-computed-modules")

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if result.Manifests[0].Type != ManifestType("typescript.cdk.aws_glue_job.python") || result.Manifests[0].Path != "job.ts" {
		t.Fatalf("unexpected manifest: %+v", result.Manifests[0])
	}
	if got := result.Manifests[0].Dependencies; len(got) != 0 {
		t.Fatalf("unexpected dependencies: %+v", got)
	}
}

func TestScanMatchesPythonGlueDependenciesWithNamespaceImport(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "glue", "job.py"), `
from aws_cdk import aws_glue as glue

glue.CfnJob(
    self,
    "Job",
    role="arn:aws:iam::123456789012:role/glue",
    command={"name": "glueetl", "python_version": "3"},
    default_arguments={
        "--job-language": "python",
        "--additional-python-modules": "pandas==2.2.1, scikit-learn==1.4.1.post1,",
    },
)
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if result.Manifests[0].Type != ManifestType("python.cdk.aws_glue_job.python") || result.Manifests[0].Path != "glue/job.py" {
		t.Fatalf("unexpected manifest: %+v", result.Manifests[0])
	}
	wantDependencies := []string{"pandas==2.2.1", "scikit-learn==1.4.1.post1"}
	if !slices.Equal(result.Manifests[0].Dependencies, wantDependencies) {
		t.Fatalf("unexpected dependencies: got %v want %v", result.Manifests[0].Dependencies, wantDependencies)
	}
}

func TestScanMatchesPythonGlueDependenciesWithNamedImportAndVariableArgs(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "job.py"), `
from aws_cdk.aws_glue import CfnJob

modules = "pandas==2.2.1"
default_args = {
    "--job-language": "python",
    "--additional-python-modules": modules,
}

CfnJob(
    self,
    "Job",
    role="arn:aws:iam::123456789012:role/glue",
    command={"name": "glueetl", "python_version": "3"},
    default_arguments=default_args,
)
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if got := result.Manifests[0].Dependencies; !slices.Equal(got, []string{"pandas==2.2.1"}) {
		t.Fatalf("unexpected dependencies: %+v", got)
	}
}

func TestScanDoesNotMatchPythonGlueWithoutAdditionalModules(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "job.py"), `
from aws_cdk import aws_glue as glue

glue.CfnJob(
    self,
    "Job",
    role="arn:aws:iam::123456789012:role/glue",
    command={"name": "glueetl", "python_version": "3"},
    default_arguments={
        "--job-language": "python",
    },
)
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 0 {
		t.Fatalf("expected no manifests, got %+v", result.Manifests)
	}
}

func TestScanMatchesPythonFixtureFromTestdata(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := filepath.Join("..", "..", "testdata", "python", "glue-cfnjob-inline")

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if result.Manifests[0].Type != ManifestType("python.cdk.aws_glue_job.python") || result.Manifests[0].Path != "job.py" {
		t.Fatalf("unexpected manifest: %+v", result.Manifests[0])
	}
	if got := result.Manifests[0].Dependencies; !slices.Equal(got, []string{"pandas==2.2.1"}) {
		t.Fatalf("unexpected dependencies: %+v", got)
	}
}

func TestScanSkipsIgnoredDirectories(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "node_modules", "package.json"), "")
	mustWriteFile(t, filepath.Join(root, "src", "package.json"), "")

	result, err := Scan(root, []string{"node_modules"}, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if result.Manifests[0].Path != "src/package.json" {
		t.Fatalf("unexpected manifest path: %+v", result.Manifests[0])
	}
}

func TestScanOverrideIgnoreListChangesTraversalBehavior(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "vendor", "pom.xml"), "")

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 1 {
		t.Fatalf("expected vendor manifest to be found without ignore list, got %d", len(result.Manifests))
	}

	result, err = Scan(root, []string{"vendor"}, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 0 {
		t.Fatalf("expected vendor manifest to be ignored, got %+v", result.Manifests)
	}
}

func TestScanRejectsFilePath(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := t.TempDir()
	filePath := filepath.Join(root, "package.json")
	mustWriteFile(t, filePath, "{}")

	_, err := Scan(filePath, nil, ruleset)
	if err == nil {
		t.Fatalf("expected error for file path")
	}
}

func TestLoadRulesRejectsMissingFields(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: \"\"\n    filename-regex: '^package\\.json$'\n"))
	if err == nil {
		t.Fatalf("expected error for missing rule name")
	}
}

func TestLoadRulesRejectsMissingFilenameRegex(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: js\n"))
	if err == nil {
		t.Fatalf("expected error for missing filename regex")
	}
}

func TestLoadRulesRejectsInvalidRegex(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: js\n    filename-regex: '('\n"))
	if err == nil {
		t.Fatalf("expected invalid regex error")
	}
}

func TestLoadRulesAcceptsBannerRegexParser(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: js-banner\n    filename-regex: '.*\\.js$'\n    banner-regex: '(?i)^/\\*!\\s*([A-Za-z0-9._/-]+)\\s+v?(\\d+\\.\\d+\\.\\d+)'\n"))
	if err != nil {
		t.Fatalf("expected banner regex rule to load: %v", err)
	}
}

func TestLoadRulesRejectsInvalidBannerRegex(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: js-banner\n    filename-regex: '.*\\.js$'\n    banner-regex: '('\n"))
	if err == nil {
		t.Fatalf("expected invalid banner regex error")
	}
}

func TestLoadRulesRejectsBannerRegexWithoutRequiredCaptureGroups(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: js-banner\n    filename-regex: '.*\\.js$'\n    banner-regex: '^/\\*!\\s*[A-Za-z0-9._/-]+\\s+v?\\d+\\.\\d+\\.\\d+'\n"))
	if err == nil {
		t.Fatalf("expected missing capture group error")
	}
}

func TestLoadRulesRejectsTerraformParserWithoutResourceType(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: terraform.aws_glue_job.python\n    filename-regex: '.*\\.tf$'\n    terraform:\n      conditions:\n        - path: default_arguments.--job-language\n          equals: python\n"))
	if err == nil {
		t.Fatalf("expected missing resource type error")
	}
}

func TestLoadRulesAcceptsTypeScriptParser(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: typescript.cdk.aws_glue_job.python\n    filename-regex: '.*\\.ts$'\n    typescript:\n      cdk_construct:\n        module: aws-cdk-lib/aws-glue\n        construct: CfnJob\n        props_argument_index: 2\n        within:\n          - defaultArguments\n        conditions:\n          - key: --additional-python-modules\n            present: true\n        extract:\n          key: --additional-python-modules\n          split: comma\n"))
	if err != nil {
		t.Fatalf("expected typescript parser to load: %v", err)
	}
}

func TestLoadRulesAcceptsPythonParser(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: python.cdk.aws_glue_job.python\n    filename-regex: '.*\\.py$'\n    python:\n      cdk_construct:\n        module: aws_cdk.aws_glue\n        construct: CfnJob\n        keyword_argument: default_arguments\n        conditions:\n          - key: --additional-python-modules\n            present: true\n        extract:\n          key: --additional-python-modules\n          split: comma\n"))
	if err != nil {
		t.Fatalf("expected python parser to load: %v", err)
	}
}

func TestLoadRulesAcceptsHTMLParser(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: html-external-scripts\n    filename-regex: '.*\\.html$'\n    html:\n      external_scripts: true\n"))
	if err != nil {
		t.Fatalf("expected html rule to load: %v", err)
	}
	if got := ruleset.SupportedManifestTypes(); !slices.Equal(got, []ManifestType{ManifestType("html-external-scripts")}) {
		t.Fatalf("unexpected supported types: %+v", got)
	}
}

func TestLoadRulesRejectsHTMLParserWithoutExternalScripts(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: html-external-scripts\n    filename-regex: '.*\\.html$'\n    html: {}\n"))
	if err == nil {
		t.Fatalf("expected missing html parser configuration error")
	}
}

func TestLoadRulesRejectsTypeScriptParserWithoutModule(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: typescript.cdk.aws_glue_job.python\n    filename-regex: '.*\\.ts$'\n    typescript:\n      cdk_construct:\n        construct: CfnJob\n        props_argument_index: 2\n        conditions:\n          - key: --additional-python-modules\n            present: true\n"))
	if err == nil {
		t.Fatalf("expected missing module error")
	}
}

func TestLoadRulesRejectsPythonParserWithoutKeywordArgument(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: python.cdk.aws_glue_job.python\n    filename-regex: '.*\\.py$'\n    python:\n      cdk_construct:\n        module: aws_cdk.aws_glue\n        construct: CfnJob\n        conditions:\n          - key: --additional-python-modules\n            present: true\n"))
	if err == nil {
		t.Fatalf("expected missing keyword argument error")
	}
}

func TestLoadRulesRejectsTypeScriptParserWithoutConstruct(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: typescript.cdk.aws_glue_job.python\n    filename-regex: '.*\\.ts$'\n    typescript:\n      cdk_construct:\n        module: aws-cdk-lib/aws-glue\n        props_argument_index: 2\n        conditions:\n          - key: --additional-python-modules\n            present: true\n"))
	if err == nil {
		t.Fatalf("expected missing construct error")
	}
}

func TestLoadRulesRejectsTerraformParserWithoutConditions(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: terraform.aws_glue_job.python\n    filename-regex: '.*\\.tf$'\n    terraform:\n      resource_type: aws_glue_job\n"))
	if err == nil {
		t.Fatalf("expected missing conditions error")
	}
}

func TestLoadRulesRejectsTerraformConditionWithoutMatcher(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: terraform.aws_glue_job.python\n    filename-regex: '.*\\.tf$'\n    terraform:\n      resource_type: aws_glue_job\n      conditions:\n        - path: default_arguments.--job-language\n"))
	if err == nil {
		t.Fatalf("expected invalid terraform condition error")
	}
}

func TestLoadRulesRejectsTypeScriptParserWithoutPropsArgumentIndex(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: typescript.cdk.aws_glue_job.python\n    filename-regex: '.*\\.ts$'\n    typescript:\n      cdk_construct:\n        module: aws-cdk-lib/aws-glue\n        construct: CfnJob\n        conditions:\n          - key: --additional-python-modules\n            present: true\n"))
	if err == nil {
		t.Fatalf("expected missing props argument index error")
	}
}

func TestLoadRulesRejectsTypeScriptParserWithoutConditions(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: typescript.cdk.aws_glue_job.python\n    filename-regex: '.*\\.ts$'\n    typescript:\n      cdk_construct:\n        module: aws-cdk-lib/aws-glue\n        construct: CfnJob\n        props_argument_index: 2\n"))
	if err == nil {
		t.Fatalf("expected missing typescript conditions error")
	}
}

func TestLoadRulesRejectsPythonParserWithoutConditions(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: python.cdk.aws_glue_job.python\n    filename-regex: '.*\\.py$'\n    python:\n      cdk_construct:\n        module: aws_cdk.aws_glue\n        construct: CfnJob\n        keyword_argument: default_arguments\n"))
	if err == nil {
		t.Fatalf("expected missing python conditions error")
	}
}

func TestLoadRulesRejectsTypeScriptConditionWithoutMatcher(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: typescript.cdk.aws_glue_job.python\n    filename-regex: '.*\\.ts$'\n    typescript:\n      cdk_construct:\n        module: aws-cdk-lib/aws-glue\n        construct: CfnJob\n        props_argument_index: 2\n        conditions:\n          - key: --additional-python-modules\n"))
	if err == nil {
		t.Fatalf("expected invalid typescript condition error")
	}
}

func TestLoadRulesRejectsTypeScriptParserWithUnsupportedExtractSplit(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: typescript.cdk.aws_glue_job.python\n    filename-regex: '.*\\.ts$'\n    typescript:\n      cdk_construct:\n        module: aws-cdk-lib/aws-glue\n        construct: CfnJob\n        props_argument_index: 2\n        conditions:\n          - key: --additional-python-modules\n            present: true\n        extract:\n          key: --additional-python-modules\n          split: space\n"))
	if err == nil {
		t.Fatalf("expected invalid extract split error")
	}
}

func TestLoadRulesRejectsPythonParserWithUnsupportedExtractSplit(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: python.cdk.aws_glue_job.python\n    filename-regex: '.*\\.py$'\n    python:\n      cdk_construct:\n        module: aws_cdk.aws_glue\n        construct: CfnJob\n        keyword_argument: default_arguments\n        conditions:\n          - key: --additional-python-modules\n            present: true\n        extract:\n          key: --additional-python-modules\n          split: space\n"))
	if err == nil {
		t.Fatalf("expected invalid python extract split error")
	}
}

func TestLoadRulesAcceptsYAMLParser(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: yaml-pip\n    filename-regex: '.*\\.ya?ml$'\n    yaml:\n      query: workflow.steps[].config.packages.pip[]\n"))
	if err != nil {
		t.Fatalf("expected yaml parser to load: %v", err)
	}
}

func TestLoadRulesRejectsYAMLParserWithoutQuery(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: yaml-pip\n    filename-regex: '.*\\.ya?ml$'\n    yaml: {}\n"))
	if err == nil {
		t.Fatalf("expected missing yaml query error")
	}
}

func TestLoadRulesRejectsMalformedYAMLQuery(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: yaml-pip\n    filename-regex: '.*\\.ya?ml$'\n    yaml:\n      query: workflow..steps[].config.packages.pip[]\n"))
	if err == nil {
		t.Fatalf("expected malformed yaml query error")
	}
}

func TestLoadRulesAcceptsTOMLParser(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: python-pyproject\n    filename-regex: '^pyproject\\.toml$'\n    toml:\n      queries:\n        - project.dependencies[]\n        - project.optional-dependencies.*[]\n"))
	if err != nil {
		t.Fatalf("expected toml parser to load: %v", err)
	}
}

func TestLoadRulesRejectsTOMLParserWithoutQueries(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: python-pyproject\n    filename-regex: '^pyproject\\.toml$'\n    toml: {}\n"))
	if err == nil {
		t.Fatalf("expected missing toml queries error")
	}
}

func TestLoadRulesRejectsMalformedTOMLQuery(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: python-pyproject\n    filename-regex: '^pyproject\\.toml$'\n    toml:\n      queries:\n        - project..dependencies[]\n"))
	if err == nil {
		t.Fatalf("expected malformed toml query error")
	}
}

func TestLoadRulesRejectsTOMLParserWithOtherParserType(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: mixed\n    filename-regex: '^pyproject\\.toml$'\n    yaml:\n      query: workflow.steps[].config.packages.pip[]\n    toml:\n      queries:\n        - project.dependencies[]\n"))
	if err == nil {
		t.Fatalf("expected multiple parser type error")
	}
}

func TestLoadRulesRejectsMultipleParserTypes(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: mixed\n    filename-regex: '.*'\n    terraform:\n      resource_type: aws_glue_job\n      conditions:\n        - path: default_arguments.--job-language\n          equals: python\n    yaml:\n      query: workflow.steps[].config.packages.pip[]\n"))
	if err == nil {
		t.Fatalf("expected multiple parser type error")
	}
}

func TestLoadRulesRejectsBannerRegexWithOtherParserType(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: mixed\n    filename-regex: '.*\\.js$'\n    banner-regex: '(?i)^/\\*!\\s*([A-Za-z0-9._/-]+)\\s+v?(\\d+\\.\\d+\\.\\d+)'\n    html:\n      external_scripts: true\n"))
	if err == nil {
		t.Fatalf("expected multiple parser type error")
	}
}

func TestLoadRulesSupportsCustomFirstMatchOrdering(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: broad\n    filename-regex: '.*\\.json$'\n  - name: specific\n    filename-regex: '^package\\.json$'\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	got, ok := ruleset.DetectManifest("package.json")
	if !ok {
		t.Fatalf("expected match")
	}
	if got != ManifestType("broad") {
		t.Fatalf("expected first pattern to win, got %q", got)
	}
}

func TestLoadDefaultRulesProvidesSupportedTypeOrder(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	want := []ManifestType{
		ManifestType("python-requirements"),
		ManifestType("python-uv"),
		ManifestType("js"),
		ManifestType("js-yarn"),
		ManifestType("java"),
		ManifestType("js-banner-block-start"),
		ManifestType("js-banner-plain-block-start"),
		ManifestType("js-banner-multiline-preserved"),
		ManifestType("js-banner-line-comment"),
		ManifestType("js-banner-version-tagged"),
		ManifestType("html-external-scripts"),
		ManifestType("terraform.aws_glue_job.python"),
		ManifestType("typescript.cdk.aws_glue_job.python"),
		ManifestType("python.cdk.aws_glue_job.python"),
	}
	got := ruleset.SupportedManifestTypes()
	if !slices.Equal(got, want) {
		t.Fatalf("unexpected supported type order: got %v want %v", got, want)
	}
}

func TestScanMatchesYAMLDependenciesFromCustomRule(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: yaml-pip\n    filename-regex: '^workflow\\.yaml$'\n    yaml:\n      query: workflow.steps[].config.packages.pip[]\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "workflow.yaml"), `
workflow:
  steps:
    - name: step1
      config:
        packages:
          pip:
            - requests
            - pendulum
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if !slices.Equal(result.Manifests[0].Dependencies, []string{"requests", "pendulum"}) {
		t.Fatalf("unexpected dependencies: %+v", result.Manifests[0].Dependencies)
	}
}

func TestScanMatchesYAMLDependenciesAcrossNestedLists(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: yaml-pip\n    filename-regex: '^workflow\\.yaml$'\n    yaml:\n      query: workflow.steps[].config.packages.pip[]\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "workflow.yaml"), `
workflow:
  steps:
    - name: step1
      config:
        packages:
          pip:
            - requests
    - name: step2
      config:
        packages:
          pip:
            - pendulum
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if !slices.Equal(result.Manifests[0].Dependencies, []string{"requests", "pendulum"}) {
		t.Fatalf("unexpected dependencies: %+v", result.Manifests[0].Dependencies)
	}
}

func TestScanDoesNotMatchYAMLWhenQueryResolvesToNonStrings(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: yaml-pip\n    filename-regex: '^workflow\\.yaml$'\n    yaml:\n      query: workflow.steps[].config.packages.pip[]\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "workflow.yaml"), `
workflow:
  steps:
    - name: step1
      config:
        packages:
          pip:
            - 123
            - true
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 0 {
		t.Fatalf("expected no manifests, got %+v", result.Manifests)
	}
}

func TestScanDoesNotMatchYAMLWhenQueryMissing(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: yaml-pip\n    filename-regex: '^workflow\\.yaml$'\n    yaml:\n      query: workflow.steps[].config.packages.pip[]\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "workflow.yaml"), `
workflow:
  jobs:
    - name: step1
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 0 {
		t.Fatalf("expected no manifests, got %+v", result.Manifests)
	}
}

func TestScanBannerRegexRequiresNonEmptyCaptureGroups(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: js-banner\n    filename-regex: '^app\\.js$'\n    banner-regex: '(?i)^/\\*!\\s*([A-Za-z0-9._/-]+)?\\s+v?(\\d+\\.\\d+\\.\\d+)?'\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "app.js"), "/*! Demo */\nconsole.log('x')\n")

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 0 {
		t.Fatalf("expected no manifests, got %+v", result.Manifests)
	}
}

func TestScanBannerRegexUsesFirstMatchingRule(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: first\n    filename-regex: '^app\\.js$'\n    banner-regex: '(?i)^/\\*!\\s*([A-Za-z0-9._/-]+)\\s+v?(\\d+\\.\\d+\\.\\d+)'\n  - name: second\n    filename-regex: '^app\\.js$'\n    banner-regex: '(?i)^/\\*!\\s*([A-Za-z0-9._/-]+)\\s+v?(\\d+\\.\\d+\\.\\d+)'\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "app.js"), "/*! jQuery v3.7.1 */\n")

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}
	if result.Manifests[0].Type != ManifestType("first") {
		t.Fatalf("expected first matching rule to win, got %+v", result.Manifests[0])
	}
}

func mustLoadDefaultRules(t *testing.T) Ruleset {
	t.Helper()
	ruleset, err := LoadDefaultRules()
	if err != nil {
		t.Fatalf("load default rules failed: %v", err)
	}
	return ruleset
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
}
