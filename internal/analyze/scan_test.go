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

	if len(result.Manifests) != 3 {
		t.Fatalf("expected 3 manifests, got %d", len(result.Manifests))
	}
	if result.Manifests[0].Path != "a/package.json" || result.Manifests[1].Path != "b/requirements-dev.txt" || result.Manifests[2].Path != "c/job.tf" {
		t.Fatalf("unexpected manifest order: %+v", result.Manifests)
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
		ManifestType("terraform.aws_glue_job.python"),
	}
	got := ruleset.SupportedManifestTypes()
	if !slices.Equal(got, want) {
		t.Fatalf("unexpected supported type order: got %v want %v", got, want)
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
