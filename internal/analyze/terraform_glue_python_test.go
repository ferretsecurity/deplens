package analyze

import (
	"strings"
	"testing"
)

func TestTerraformGluePythonExtractsEveryJobWithPrecedence(t *testing.T) {
	parser, err := newTerraformGluePythonParser(terraformGluePythonConfig{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := parser.Analyze("jobs.tf", []byte(`
resource "aws_glue_job" "default_language" {
  default_arguments = {
    "--additional-python-modules" = "pandas==2.2.1,paramiko"
    "--enable-continuous-cloudwatch-log" = var.logging
  }
}
resource "aws_glue_job" "override" {
  default_arguments = {
    "--job-language" = "python"
    "--additional-python-modules" = "old==1"
  }
  non_overridable_arguments = {
    "--additional-python-modules" = "new==2"
  }
}
resource "aws_glue_job" "scala" {
  default_arguments = {
    "--job-language" = "scala"
    "--additional-python-modules" = "ignored"
  }
}
`))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Recognized || result.Analysis.Extraction != ExtractionComplete || len(result.Groups) != 2 {
		t.Fatalf("result: %+v", result)
	}
	if got := result.Groups[0]; got.Name != "default_language" || got.State != GroupReady || len(got.Dependencies) != 2 {
		t.Fatalf("first group: %+v", got)
	}
	if got := result.Groups[1]; got.Name != "override" || len(got.Dependencies) != 1 || got.Dependencies[0].Raw != "new==2" {
		t.Fatalf("override group: %+v", got)
	}
}

func TestTerraformGluePythonReportsRelevantDynamicValues(t *testing.T) {
	parser, _ := newTerraformGluePythonParser(terraformGluePythonConfig{})
	for name, source := range map[string]string{
		"language": `resource "aws_glue_job" "job" { default_arguments = { "--job-language" = var.language } }`,
		"modules":  `resource "aws_glue_job" "job" { default_arguments = { "--additional-python-modules" = local.modules } }`,
		"override map": `resource "aws_glue_job" "job" {
  default_arguments = { "--additional-python-modules" = "pandas" }
  non_overridable_arguments = var.args
}`,
		"installer": `resource "aws_glue_job" "job" {
  default_arguments = {
    "--additional-python-modules" = "pandas"
    "--python-modules-installer-option" = var.options
  }
}`,
	} {
		t.Run(name, func(t *testing.T) {
			result, err := parser.Analyze("job.tf", []byte(source))
			if err != nil {
				t.Fatal(err)
			}
			if !result.Recognized || result.Analysis.Extraction != ExtractionFailed || len(result.Diagnostics) == 0 || !strings.Contains(result.Diagnostics[0].Message, `Glue job "job"`) {
				t.Fatalf("result: %+v", result)
			}
		})
	}
}

func TestTerraformGluePythonRejectsUnsupportedDeclarations(t *testing.T) {
	parser, _ := newTerraformGluePythonParser(terraformGluePythonConfig{})
	for name, declaration := range map[string]string{
		"git":               `"--additional-python-modules" = "pkg @ git+https://example.test/pkg.git"`,
		"url":               `"--additional-python-modules" = "https://example.test/pkg.whl"`,
		"path":              `"--additional-python-modules" = "./pkg.whl"`,
		"repository":        `"--additional-python-modules" = "pandas", "--python-modules-installer-option" = "--extra-index-url https://example.test/simple"`,
		"requirements file": `"--additional-python-modules" = "-r requirements.txt"`,
	} {
		t.Run(name, func(t *testing.T) {
			source := `resource "aws_glue_job" "job" { default_arguments = { ` + declaration + ` } }`
			result, err := parser.Analyze("job.tf", []byte(source))
			if err != nil {
				t.Fatal(err)
			}
			if result.Analysis.Extraction != ExtractionFailed || len(result.Diagnostics) == 0 {
				t.Fatalf("result: %+v", result)
			}
		})
	}
}

func TestTerraformGluePythonKeepsMissingAndEmptyGroups(t *testing.T) {
	parser, _ := newTerraformGluePythonParser(terraformGluePythonConfig{})
	result, err := parser.Analyze("jobs.tf", []byte(`
resource "aws_glue_job" "missing" {}
resource "aws_glue_job" "empty" { default_arguments = { "--additional-python-modules" = "" } }
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Groups) != 2 || result.Groups[0].State != GroupMissing || result.Groups[1].State != GroupEmpty {
		t.Fatalf("groups: %+v", result.Groups)
	}
}

func TestTerraformGluePythonPreservesRequirementCommas(t *testing.T) {
	parser, _ := newTerraformGluePythonParser(terraformGluePythonConfig{})
	result, err := parser.Analyze("job.tf", []byte(`resource "aws_glue_job" "job" {
  default_arguments = { "--additional-python-modules" = "requests[security,socks]>=2,<3,paramiko" }
}`))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"requests[security,socks]>=2,<3", "paramiko"}
	if len(result.Dependencies) != len(want) {
		t.Fatalf("dependencies: %+v", result.Dependencies)
	}
	for index := range want {
		if result.Dependencies[index].Raw != want[index] {
			t.Fatalf("dependency %d: %q, want %q", index, result.Dependencies[index].Raw, want[index])
		}
	}
}
