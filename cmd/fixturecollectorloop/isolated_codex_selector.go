package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"
)

const (
	selectorOutputLimit = 64 << 10
)

// IsolatedCodexSelectorConfig is reviewed, rather than inherited from a
// user's Codex configuration. Changing it intentionally changes the decision
// fingerprint recorded in progress.
type IsolatedCodexSelectorConfig struct {
	Executable      string
	Model           string
	ReasoningEffort string
}

func defaultIsolatedCodexSelectorConfig() IsolatedCodexSelectorConfig {
	return IsolatedCodexSelectorConfig{
		Executable: "codex", Model: "gpt-5.4", ReasoningEffort: "high",
	}
}

type isolatedCodexSelector struct {
	config     IsolatedCodexSelectorConfig
	cliVersion string
}

func newIsolatedCodexSelector(config IsolatedCodexSelectorConfig) *isolatedCodexSelector {
	if config.Executable == "" {
		config.Executable = "codex"
	}
	return &isolatedCodexSelector{config: config}
}

func (s *isolatedCodexSelector) configurationFingerprint() string {
	version := s.cliVersion
	if version == "" {
		version = "unverified"
	}
	return hash(strings.Join([]string{"isolated-codex-selector-v2", version, s.config.Model, s.config.ReasoningEffort, strings.Join(disabledCodexFeatures, ",")}, "\x00"))
}

// disabledCodexFeatures must be available in the installed CLI so the selector
// can explicitly disable every capability outside its narrow protocol. Version
// numbers are unrestricted; feature availability defines compatibility.
var disabledCodexFeatures = []string{
	"shell_tool", "unified_exec", "shell_snapshot", "hooks", "multi_agent",
	"apps", "plugins", "remote_plugin", "plugin_sharing", "tool_suggest",
	"skill_search", "skill_mcp_dependency_install", "in_app_browser", "browser_use",
	"browser_use_external", "browser_use_full_cdp_access", "computer_use",
	"image_generation", "goals", "guardian_approval", "workspace_dependencies",
	"auth_elicitation", "tool_call_mcp_elicitation", "in_app_updates",
}

func (s *isolatedCodexSelector) Preflight() error {
	if s.config.Model == "" || s.config.ReasoningEffort == "" {
		return errors.New("isolated Codex selector model and reasoning effort must be configured")
	}
	version, err := exec.Command(s.config.Executable, "--version").Output()
	cliVersion := strings.TrimSpace(string(version))
	if err != nil || !strings.HasPrefix(cliVersion, "codex-cli ") || strings.TrimSpace(strings.TrimPrefix(cliVersion, "codex-cli ")) == "" {
		return errors.New("installed Codex CLI is unavailable")
	}
	s.cliVersion = cliVersion
	features, err := exec.Command(s.config.Executable, "features", "list").Output()
	if err != nil {
		return errors.New("cannot list installed Codex isolation features")
	}
	available := featureSet(string(features))
	for _, feature := range disabledCodexFeatures {
		if !available[feature] {
			return fmt.Errorf("installed Codex lacks required isolation feature %q", feature)
		}
	}
	if err := exec.Command(s.config.Executable, "login", "status").Run(); err != nil {
		return errors.New("Codex OAuth session is unavailable")
	}
	return nil
}

func featureSet(output string) map[string]bool {
	set := make(map[string]bool)
	for _, token := range strings.FieldsFunc(output, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-')
	}) {
		set[token] = true
	}
	return set
}

func (s *isolatedCodexSelector) Select(ctx context.Context, _ Iteration, packet []byte) (ResearchResult, error) {
	workDir, err := os.MkdirTemp("", "deplens-selector-work-")
	if err != nil {
		return ResearchResult{}, err
	}
	defer os.RemoveAll(workDir)
	if err := os.Chmod(workDir, 0o700); err != nil {
		return ResearchResult{}, err
	}
	controlDir, err := os.MkdirTemp("", "deplens-selector-control-")
	if err != nil {
		return ResearchResult{}, err
	}
	defer os.RemoveAll(controlDir)
	if err := os.Chmod(controlDir, 0o700); err != nil {
		return ResearchResult{}, err
	}
	schemaPath := filepath.Join(controlDir, "selection.schema.json")
	if err := os.WriteFile(schemaPath, []byte(selectionSchema), 0o600); err != nil {
		return ResearchResult{}, err
	}

	cmd := exec.CommandContext(ctx, s.config.Executable, s.arguments(workDir, schemaPath)...)
	cmd.Dir = workDir
	cmd.Env = selectorEnvironment()
	cmd.Stdin = bytes.NewReader(packet)
	var stdout, stderr boundedBuffer
	stdout.limit, stderr.limit = selectorOutputLimit, selectorOutputLimit
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return ResearchResult{}, errors.New("isolated Codex selector invocation failed")
	}
	if stdout.exceeded || stderr.exceeded {
		return ResearchResult{}, errors.New("isolated Codex selector output exceeded its memory limit")
	}
	selection, err := parseSelection(stdout.Bytes())
	if err != nil {
		return ResearchResult{}, fmt.Errorf("isolated Codex selector returned invalid structured output: %w", err)
	}
	return ResearchResult{Selection: selection}, nil
}

func (s *isolatedCodexSelector) arguments(workDir, schemaPath string) []string {
	args := []string{"exec", "--ephemeral", "--ignore-user-config", "--ignore-rules", "--strict-config", "--skip-git-repo-check", "--cd", workDir, "--model", s.config.Model, "--output-schema", schemaPath}
	for _, feature := range disabledCodexFeatures {
		args = append(args, "--disable", feature)
	}
	readRoot := fmt.Sprintf("permissions.fixture-selector.filesystem={%q=\"read\"}", workDir)
	args = append(args,
		"--config", `web_search="disabled"`, "--config", `approval_policy="never"`,
		"--config", `default_permissions="fixture-selector"`, "--config", readRoot,
		"--config", `permissions.fixture-selector.network.enabled=false`,
		"--config", `shell_environment_policy.inherit="none"`, "--config", `allow_login_shell=false`,
		"--config", `project_doc_max_bytes=0`, "--config", `skills.include_instructions=false`,
		"--config", `analytics.enabled=false`, "--config", fmt.Sprintf("model_reasoning_effort=%q", s.config.ReasoningEffort), "-")
	return args
}

func selectorEnvironment() []string {
	keys := []string{"PATH", "CODEX_HOME", "HOME", "SSL_CERT_FILE", "SSL_CERT_DIR", "LANG", "LC_ALL"}
	env := make([]string, 0, len(keys))
	for _, key := range keys {
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}
	return env
}

type boundedBuffer struct {
	bytes.Buffer
	limit    int
	exceeded bool
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.limit {
		remaining := b.limit - b.Len()
		if remaining > 0 {
			_, _ = b.Buffer.Write(p[:remaining])
		}
		b.exceeded = true
		return len(p), nil
	}
	return b.Buffer.Write(p)
}

func parseSelection(data []byte) (Selection, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return Selection{}, err
	}
	rawSelected, ok := fields["selected"]
	if !ok || string(rawSelected) == "null" {
		return Selection{}, errors.New("missing selected array")
	}
	if len(fields) != 1 {
		return Selection{}, errors.New("unexpected output field")
	}
	var selection Selection
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&selection); err != nil {
		return Selection{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return Selection{}, errors.New("multiple JSON values")
	}
	return selection, nil
}

const selectionSchema = `{"type":"object","additionalProperties":false,"required":["selected"],"properties":{"selected":{"type":"array","minItems":0,"maxItems":5,"items":{"type":"object","additionalProperties":false,"required":["id","rationale"],"properties":{"id":{"type":"string"},"rationale":{"type":"string"}}}}}}`
