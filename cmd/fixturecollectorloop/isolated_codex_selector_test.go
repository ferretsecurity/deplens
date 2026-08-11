package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsolatedCodexSelectorUsesPinnedIsolatedStdinProtocol(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	logPath := filepath.Join(root, "invocation")
	fake := filepath.Join(root, "codex")
	script := fmt.Sprintf(`#!/bin/sh
if [ "$1" = "--version" ]; then echo 'codex-cli 0.147.0'; exit 0; fi
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
printf '{"selected":[{"id":"candidate-1","rationale":"varied example"}]}'
`, logPath, logPath, logPath, logPath, logPath, logPath)
	if err := os.WriteFile(fake, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	selector := newIsolatedCodexSelector(IsolatedCodexSelectorConfig{Executable: fake, Model: "gpt-5.4", ReasoningEffort: "high"})
	if err := selector.Preflight(); err != nil {
		t.Fatalf("Preflight() error = %v", err)
	}
	result, err := selector.Select(context.Background(), Iteration{}, []byte(`{"packet":"untrusted source contents"}`))
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if got := result.Selection.Selected; len(got) != 1 || got[0].ID != "candidate-1" {
		t.Fatalf("selection = %#v", got)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(log)
	for _, want := range []string{"--ephemeral", "--ignore-user-config", "--ignore-rules", "--strict-config", "--output-schema", "--disable shell_tool", "--config approval_policy=\"never\"", "--config web_search=\"disabled\"", "--model gpt-5.4", "model_reasoning_effort=\"high\"", " -"} {
		if !strings.Contains(text, want) {
			t.Errorf("invocation missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "untrusted source contents") && !strings.Contains(text, "STDIN={\"packet\":\"untrusted source contents\"}") {
		t.Errorf("packet appeared outside stdin: %s", text)
	}
	for _, forbidden := range []string{"GITHUB_TOKEN=", "GH_TOKEN=", "SSH_AUTH_SOCK=", "AWS_SECRET_ACCESS_KEY="} {
		if strings.Contains(text, forbidden) {
			t.Errorf("child inherited forbidden environment %q", forbidden)
		}
	}
}

func TestIsolatedCodexSelectorFailsClosedForMissingFeature(t *testing.T) {
	selector := newIsolatedCodexSelector(IsolatedCodexSelectorConfig{Executable: filepath.Join(t.TempDir(), "missing")})
	if err := selector.Preflight(); err == nil {
		t.Fatal("Preflight() succeeded for unavailable executable")
	}
}

func TestParseSelectionRequiresTheSchemaShape(t *testing.T) {
	for _, output := range []string{`{}`, `{"selected":null}`, `{"selected":[],"invented":true}`, `{"selected":[]} {"selected":[]}`} {
		if _, err := parseSelection([]byte(output)); err == nil {
			t.Errorf("parseSelection(%s) succeeded", output)
		}
	}
}
