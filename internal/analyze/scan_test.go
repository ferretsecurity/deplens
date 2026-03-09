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
		{name: "my-requirements.txt", want: ManifestType("python-requirements")},
		{name: "requirements-dev.txt", want: ManifestType("python-requirements")},
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
		"requirementsdev.txt",
		"requirements.txt.backup",
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
	mustWriteFile(t, filepath.Join(root, "b", "requirements-dev.txt"), "")
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
	if result.Manifests[0].Path != "a/index.html" || result.Manifests[1].Path != "a/package.json" || result.Manifests[2].Path != "b/requirements-dev.txt" || result.Manifests[3].Path != "c/job.tf" {
		t.Fatalf("unexpected manifest order: %+v", result.Manifests)
	}
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

func TestLoadRulesRejectsTerraformParserWithoutResourceType(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: terraform.aws_glue_job.python\n    filename-regex: '.*\\.tf$'\n    terraform:\n      conditions:\n        - path: default_arguments.--job-language\n          equals: python\n"))
	if err == nil {
		t.Fatalf("expected missing resource type error")
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

func TestLoadRulesRejectsMultipleParserTypes(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: mixed\n    filename-regex: '.*'\n    terraform:\n      resource_type: aws_glue_job\n      conditions:\n        - path: default_arguments.--job-language\n          equals: python\n    yaml:\n      query: workflow.steps[].config.packages.pip[]\n"))
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
		ManifestType("html-external-scripts"),
		ManifestType("terraform.aws_glue_job.python"),
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
