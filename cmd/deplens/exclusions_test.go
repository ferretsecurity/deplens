package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ferretsecurity/deplens/internal/analyze"
)

func TestRunExclusionComposition(t *testing.T) {
	dir := t.TempDir()
	project := filepath.Join(dir, "project")
	writeFile(t, filepath.Join(project, "go.mod"), "module example.com/app\n\ngo 1.23\nrequire example.com/dep v1.0.0\n")
	extension := filepath.Join(dir, "extension.yaml")
	writeFile(t, extension, "rules:\n"+extensionDetector("replacement", "^go\\.mod$")+"checks:\n"+extensionCheck("custom"))
	for _, args := range [][]string{
		{"--disable-rule", "go-mod", "--extend-rules", extension},
		{"--extend-rules", extension, "--disable-rule", "go-mod", "--disable-rule", "go-mod"},
	} {
		result := runComposedJSON(t, append(args, project)...)
		if len(result.Sources) != 1 || result.Sources[0].Detector != "replacement" {
			t.Fatalf("sources: %+v", result.Sources)
		}
		assertDisabledCheck(t, result, "custom", "go-mod")
	}
	result := runComposedJSON(t, "--extend-rules", extension, "--disable-check", "custom", "--disable-check", "go-sum-missing", "--disable-check", "custom", project)
	if len(result.Sources) != 1 || result.Sources[0].Detector != "go-mod" {
		t.Fatal(result.Sources)
	}
	for _, r := range result.CheckRuns {
		if r.CheckID == "custom" || r.CheckID == "go-sum-missing" {
			t.Fatal(r)
		}
	}
	for _, f := range result.Findings {
		if f.CheckID == "custom" || f.CheckID == "go-sum-missing" {
			t.Fatal(f)
		}
	}
}

func assertDisabledCheck(t *testing.T, result analyze.ScanResult, id analyze.CheckID, detector string) {
	t.Helper()
	found := false
	for _, r := range result.CheckRuns {
		if r.CheckID == id {
			found = true
			if r.Status != analyze.CheckSkipped || r.ReasonCode != "detector-disabled" || !strings.Contains(r.Detail, detector) {
				t.Fatalf("run: %+v", r)
			}
		}
	}
	if !found {
		t.Fatalf("no skip for %s: %+v", id, result.CheckRuns)
	}
	for _, f := range result.Findings {
		if f.CheckID == id {
			t.Fatalf("false finding: %+v", f)
		}
	}
}

func TestRunExclusionPrerequisites(t *testing.T) {
	for _, tc := range []struct {
		name, detector string
		files          map[string]string
		check          analyze.CheckID
	}{
		{"go lock", "go-sum", map[string]string{"app/go.mod": "module example.com/app\n\ngo 1.23\nrequire example.com/dep v1.0.0\n", "app/go.sum": "example.com/dep v1.0.0 h1:abc\n"}, "go-sum-missing"},
		{"npm lock", "js-npm-lock", map[string]string{"package.json": `{"dependencies":{"a":"1"},"packageManager":"npm@10.0.0"}`, "package-lock.json": `{"lockfileVersion":3,"packages":{"node_modules/a":{"version":"1"}}}`}, "javascript-npm-lockfile-missing"},
		{"competing manager", "js-yarn", map[string]string{"package.json": `{"dependencies":{"a":"1"},"packageManager":"npm@10.0.0"}`, "yarn.lock": "a@1:\n  version \"1\"\n"}, "javascript-npm-lockfile-missing"},
		{"manager discovery", "js-yarnrc", map[string]string{"package.json": `{"dependencies":{"a":"1"}}`, ".yarnrc.yml": "nodeLinker: node-modules\n"}, "javascript-yarn-lockfile-missing"},
		{"workspace", "js-pnpm-workspace", map[string]string{"packages/app/package.json": `{"dependencies":{"a":"1"},"packageManager":"pnpm@9.0.0"}`, "pnpm-workspace.yaml": "packages:\n  - packages/*\n", "pnpm-lock.yaml": "lockfileVersion: '9.0'\n"}, "javascript-pnpm-lockfile-missing"},
		{"discovery", "js", map[string]string{"package.json": `{"dependencies":{"a":"1"},"packageManager":"npm@10.0.0"}`}, "javascript-npm-lockfile-missing"},
		{"uv discovery", "python-pyproject", map[string]string{"pyproject.toml": "[project]\nname='app'\ndependencies=['a']\n"}, "python-uv-lockfile-missing"},
		{"accepted alternative", "js-npm-lock", map[string]string{"package.json": `{"dependencies":{"a":"1"},"packageManager":"npm@10.0.0"}`, "npm-shrinkwrap.json": `{"lockfileVersion":3,"packages":{"node_modules/a":{"version":"1"}}}`}, "javascript-npm-lockfile-missing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for file, content := range tc.files {
				writeFile(t, filepath.Join(dir, file), content)
			}
			writeFile(t, filepath.Join(dir, "other/composer.json"), `{"type":"project","require":{"vendor/pkg":"1"}}`)
			result := runComposedJSON(t, "--disable-rule", tc.detector, dir)
			assertDisabledCheck(t, result, tc.check, tc.detector)
			unaffected := false
			for _, f := range result.Findings {
				if f.CheckID == "php-composer-lockfile-missing-for-application" {
					unaffected = true
				}
			}
			if !unaffected {
				t.Fatalf("unrelated check suppressed: %+v", result)
			}
		})
	}
}

func TestRunExclusionErrorsAndEmpty(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "base.yaml")
	writeFile(t, base, "rules:\n"+extensionDetector("only", "^input$")+"checks:\n"+extensionCheck("policy"))
	writeFile(t, filepath.Join(dir, "input"), "data")
	for _, flag := range []string{"--disable-rule", "--disable-check"} {
		for _, id := range []string{"missing", "ONLY", "only,missing", "*", "go-mod", ""} {
			var out, stderr bytes.Buffer
			if code := run([]string{"--rules", base, flag, id, filepath.Join(dir, "nonexistent")}, &out, &stderr); code == 0 || !strings.Contains(stderr.String(), "unknown") {
				t.Fatalf("%s %q: %d %s", flag, id, code, &stderr)
			}
		}
	}
	result := runComposedJSON(t, "--rules", base, "--disable-rule", "only", "--disable-check", "policy", dir)
	if result.Sources == nil || result.CheckRuns == nil || result.Findings == nil || len(result.Sources)+len(result.CheckRuns)+len(result.Findings) != 0 {
		t.Fatalf("empty: %+v", result)
	}
	invalid := filepath.Join(dir, "invalid.yaml")
	writeFile(t, invalid, "rules:\n"+extensionDetector("bad", "["))
	duplicate := filepath.Join(dir, "duplicate.yaml")
	writeFile(t, duplicate, "rules:\n"+extensionDetector("only", "^input$"))
	for _, file := range []string{invalid, duplicate} {
		id := "only"
		if file == invalid {
			id = "bad"
		}
		var out, stderr bytes.Buffer
		if code := run([]string{"--rules", base, "--extend-rules", file, "--disable-rule", id, dir}, &out, &stderr); code == 0 {
			t.Fatal("invalid excluded definition accepted")
		}
	}
}

func TestRunCustomBaseAbsenceIsNotExclusion(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "base.yaml")
	writeFile(t, base, "rules:\n  - id: go-mod\n    form: manifest\n    roles: [declaration]\n    filename-regex: '^go\\.mod$'\n    analyzer: {type: go-mod}\nchecks:\n"+extensionCheck("custom"))
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n\ngo 1.23\nrequire example.com/dep v1.0.0\n")
	writeFile(t, filepath.Join(dir, "go.sum"), "example.com/dep v1.0.0 h1:abc\n")
	result := runComposedJSON(t, "--rules", base, dir)
	if len(result.Findings) != 1 || result.Findings[0].CheckID != "custom" {
		t.Fatalf("custom-base baseline: %+v", result)
	}
}

func TestRunDisabledCheckHumanOutput(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n\ngo 1.23\nrequire example.com/dep v1.0.0\n")
	var out, stderr bytes.Buffer
	if code := run([]string{"--disable-rule", "go-mod", dir}, &out, &stderr); code != 0 || !strings.Contains(out.String(), "[skipped] go-sum-missing") || !strings.Contains(out.String(), "prerequisite detectors disabled: go-mod") {
		t.Fatalf("human exit=%d out=%s err=%s", code, &out, &stderr)
	}
	result := runComposedJSON(t, "--disable-rule", "go-mod", dir)
	assertDisabledCheck(t, result, "dependency-source-codeowners-missing", "go-mod")
}

func TestRunAllBuiltinDetectorsExcluded(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{"dependencies":{"a":"1"}}`)
	rules, err := analyze.LoadDefaultRules()
	if err != nil {
		t.Fatal(err)
	}
	args := []string{}
	for _, id := range rules.DetectorIDs() {
		args = append(args, "--disable-rule", string(id))
	}
	result := runComposedJSON(t, append(args, dir)...)
	if result.Sources == nil || len(result.Sources) != 0 || result.Findings == nil || len(result.Findings) != 0 || len(result.CheckRuns) == 0 {
		t.Fatalf("all excluded: %+v", result)
	}
	for _, r := range result.CheckRuns {
		if r.Status != analyze.CheckSkipped || r.ReasonCode != "detector-disabled" {
			t.Fatalf("run: %+v", r)
		}
	}
}
