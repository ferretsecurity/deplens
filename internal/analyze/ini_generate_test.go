package analyze

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSetupCfgGenerationPreservesIndependentDeclarations(t *testing.T) {
	content, err := os.ReadFile("../../testdata/python/setup-cfg-generation/setup.cfg")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "setup.cfg"), content, 0600); err != nil {
		t.Fatal(err)
	}
	result, err := Scan(root, nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 1 {
		t.Fatalf("ordinary scan wrote files: %v", entries)
	}
	if err := GeneratePythonRequirements(&result, false); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"options.install_requires":      "requests[security,socks]>=2.31,<3; python_version >= \"3.9\"\nurllib3<2\n",
		"options.setup_requires":        "wheel>=0.42\nsetuptools_scm>=8\n",
		"options.extras_require.modern": "urllib3>=2,<3; python_version >= \"3.10\"\n",
		"options.extras_require.legacy": "urllib3<2\n",
	}
	if len(result.Generation.Paths) != len(want) {
		t.Fatalf("generation: %+v", result.Generation)
	}
	for name, content := range want {
		path := "setup.cfg-" + name + ".generated-requirements.txt"
		got, err := os.ReadFile(filepath.Join(root, path))
		if err != nil || string(got) != content {
			t.Errorf("%s: got %q, err %v, want %q", path, got, err, content)
		}
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var output struct {
		Generation GenerationResult `json:"generation"`
	}
	if err := json.Unmarshal(encoded, &output); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(output.Generation, *result.Generation) {
		t.Fatalf("JSON generation: %s", encoded)
	}
	for _, outcome := range output.Generation.Outcomes {
		if outcome.Group != outcome.Location || outcome.Source != "setup.cfg" {
			t.Fatalf("outcome: %+v", outcome)
		}
		if outcome.Group == "options.extras_require.empty" && (outcome.Status != GenerationOutcomeStatus(GroupEmpty) || outcome.Path != "") {
			t.Fatalf("empty outcome: %+v", outcome)
		}
	}
	if err := GeneratePythonRequirements(&result, false); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("repeat error: %v", err)
	}
	if err := GeneratePythonRequirements(&result, true); err != nil {
		t.Fatal(err)
	}
}

func TestSetupCfgGenerationRejectsIncompleteInputBeforeWriting(t *testing.T) {
	for name, value := range map[string]string{
		"file":                   "file: requirements.txt",
		"interpolation":          "%(requirements)s",
		"embedded interpolation": "requests==%(version)s",
		"mixed":                  "\n    requests\n    file: requirements.txt",
		"comma packages":         "requests>=2, urllib3<3",
		"url":                    "pkg @ https://example.test/pkg.whl",
		"pip option":             "--index-url https://example.test/simple",
		"inline marker":          `requests ; python_version < "3.13"`,
		"invalid marker":         "\n    requests; unknown_variable == 'yes'",
		"last empty duplicate":   "requests\ninstall_requires =",
		"first empty duplicate":  "\ninstall_requires = requests",
		"quoted duplicate":       "requests\n\"install_requires\" = urllib3",
		"same duplicate":         "requests\ninstall_requires = requests",
		"duplicate":              "requests\ninstall_requires = urllib3",
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "setup.cfg"), []byte("[options]\nsetup_requires = wheel\ninstall_requires = "+value+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			result, err := Scan(root, nil, mustLoadDefaultRules(t))
			if err != nil {
				t.Fatal(err)
			}
			err = GeneratePythonRequirements(&result, false)
			if err == nil {
				t.Fatalf("incomplete export accepted: %+v", result)
			}
			entries, _ := os.ReadDir(root)
			if len(entries) != 1 {
				t.Fatalf("preflight wrote files: %v", entries)
			}
		})
	}
}

func TestSetupCfgGenerationMissingAndEmpty(t *testing.T) {
	for name, content := range map[string]string{
		"metadata only": "[metadata]\nname = demo\n",
		"empty":         "[options]\ninstall_requires =\n[options.extras_require]\n",
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "setup.cfg"), []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			result, err := Scan(root, nil, mustLoadDefaultRules(t))
			if err != nil {
				t.Fatal(err)
			}
			if err := GeneratePythonRequirements(&result, false); err != nil {
				t.Fatal(err)
			}
			if len(result.Generation.Paths) != 0 || len(result.Generation.Outcomes) != 3 {
				t.Fatalf("generation: %+v", result.Generation)
			}
			for _, outcome := range result.Generation.Outcomes {
				want := GenerationOutcomeStatus(GroupMissing)
				if name == "empty" && outcome.Group != "options.setup_requires" {
					want = GenerationOutcomeStatus(GroupEmpty)
				}
				if outcome.Status != want || outcome.Path != "" {
					t.Fatalf("outcome: %+v, want %s", outcome, want)
				}
			}
		})
	}
}

func TestSetupCfgGenerationIgnoresInlineCommentSemicolons(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "setup.cfg"), []byte("[options]\ninstall_requires = requests # comment; fakepkg\n"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := Scan(root, nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := GeneratePythonRequirements(&result, false); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, result.Generation.Paths[0]))
	if err != nil || string(got) != "requests\n" {
		t.Fatalf("content %q, err %v", got, err)
	}
}

func TestSetupCfgGenerationDisambiguatesExtraNames(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "setup.cfg"), []byte("[options.extras_require]\na/b = requests<2\na-b = requests>=2\n"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := Scan(root, nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := GeneratePythonRequirements(&result, false); err != nil {
		t.Fatal(err)
	}
	if len(result.Generation.Paths) != 2 || result.Generation.Paths[0] == result.Generation.Paths[1] {
		t.Fatalf("generation: %+v", result.Generation)
	}
	for _, path := range result.Generation.Paths {
		if !strings.Contains(path, "-at-") {
			t.Fatalf("no stable collision suffix: %s", path)
		}
	}
}

func TestSetupCfgGenerationRejectsConditionalExtraKeys(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "setup.cfg"), []byte("[options.extras_require]\ntest:python_version < '3.10' = requests\n"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := Scan(root, nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := GeneratePythonRequirements(&result, false); err == nil {
		t.Fatal("conditional extra exported without its condition")
	}
}

func TestSetupCfgGenerationRejectsInitiallyIndentedDuplicate(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "setup.cfg"), []byte("[options]\n  install_requires = requests\ninstall_requires = urllib3\n"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := Scan(root, nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := GeneratePythonRequirements(&result, false); err == nil {
		t.Fatal("duplicate declaration exported")
	}
}
