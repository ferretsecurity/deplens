package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsolatedCodexSelectorUsesCompatibleIsolatedStdinProtocol(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	logPath := filepath.Join(root, "invocation")
	fake := filepath.Join(root, "codex")
	script := fmt.Sprintf(`#!/bin/sh
if [ "$1" = "--version" ]; then echo 'codex-cli 999.123.45-custom'; exit 0; fi
if [ "$1" = "features" ]; then
  cat <<'EOF'
shell_tool unified_exec shell_snapshot hooks multi_agent apps plugins remote_plugin plugin_sharing tool_suggest skill_search skill_mcp_dependency_install in_app_browser browser_use browser_use_external browser_use_full_cdp_access computer_use image_generation goals guardian_approval workspace_dependencies auth_elicitation tool_call_mcp_elicitation in_app_updates
EOF
  exit 0
fi
printf 'ARGS=%%s\n' "$*" > %q
printf 'ENV=%%s\n' "$*" >> %q
env | sort >> %q
printf '\nSTDIN=' >> %q
cat >> %q
printf '\n' >> %q
printf '{"selected":[{"id":"candidate-1","rationale":"common usage"},{"id":"candidate-2","rationale":"structural variation"},{"id":"candidate-3","rationale":"edge case"}]}'
`, logPath, logPath, logPath, logPath, logPath, logPath)
	if err := os.WriteFile(fake, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	selector := newIsolatedCodexSelector(IsolatedCodexSelectorConfig{Executable: fake, Model: "gpt-5.6-terra", ReasoningEffort: "medium"})
	if err := selector.Preflight(); err != nil {
		t.Fatalf("Preflight() error = %v", err)
	}
	if selector.cliVersion != "codex-cli 999.123.45-custom" {
		t.Fatalf("recorded CLI version = %q", selector.cliVersion)
	}
	otherVersion := newIsolatedCodexSelector(selector.config)
	otherVersion.cliVersion = "codex-cli 1.0.0"
	if selector.configurationFingerprint() == otherVersion.configurationFingerprint() {
		t.Fatal("selector fingerprint did not include the observed CLI version")
	}
	result, err := selector.Select(context.Background(), Iteration{}, []byte(`{"packet":"untrusted source contents"}`))
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if got := result.Selection.Selected; len(got) != selectionCount || got[0].ID != "candidate-1" {
		t.Fatalf("selection = %#v", got)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(log)
	for _, want := range []string{"--ephemeral", "--ignore-user-config", "--ignore-rules", "--strict-config", "--output-schema", "--disable shell_tool", "--config approval_policy=\"never\"", "--config web_search=\"disabled\"", "--model gpt-5.6-terra", "model_reasoning_effort=\"medium\"", " -"} {
		if !strings.Contains(text, want) {
			t.Errorf("invocation missing %q:\n%s", want, text)
		}
	}
	if !strings.Contains(text, "Select exactly three candidates") || !strings.Contains(text, "most common real-world usage") {
		t.Errorf("selector instructions missing from stdin: %s", text)
	}
	if strings.Contains(text, "untrusted source contents") && !strings.Contains(text, "SELECTION_PACKET_JSON\n{\"packet\":\"untrusted source contents\"}") {
		t.Errorf("packet appeared outside stdin: %s", text)
	}
	if !strings.Contains(selectionSchema, `"minItems":3,"maxItems":3`) {
		t.Fatalf("selection schema does not require exactly three items: %s", selectionSchema)
	}
	for _, forbidden := range []string{"GITHUB_TOKEN=", "GH_TOKEN=", "SSH_AUTH_SOCK=", "AWS_SECRET_ACCESS_KEY="} {
		if strings.Contains(text, forbidden) {
			t.Errorf("child inherited forbidden environment %q", forbidden)
		}
	}
}

func TestDefaultIsolatedCodexSelectorUsesReviewedModel(t *testing.T) {
	config := defaultIsolatedCodexSelectorConfig()
	if config.Model != "gpt-5.6-terra" || config.ReasoningEffort != "medium" {
		t.Fatalf("default selector config = %+v", config)
	}
}

func TestIsolatedCodexSelectorFailsClosedForMissingFeature(t *testing.T) {
	selector := newIsolatedCodexSelector(IsolatedCodexSelectorConfig{Executable: filepath.Join(t.TempDir(), "missing")})
	if err := selector.Preflight(); err == nil {
		t.Fatal("Preflight() succeeded for unavailable executable")
	}
}

func TestParseSelectionRequiresTheSchemaShape(t *testing.T) {
	for _, output := range []string{
		`{}`,
		`{"selected":null}`,
		`{"selected":[]}`,
		`{"selected":[{"id":"one","rationale":"only one"}]}`,
		`{"selected":[],"invented":true}`,
		`{"selected":[]} {"selected":[]}`,
	} {
		if _, err := parseSelection([]byte(output)); err == nil {
			t.Errorf("parseSelection(%s) succeeded", output)
		}
	}
}
