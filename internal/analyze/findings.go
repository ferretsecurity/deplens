package analyze

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

type javascriptManifest struct {
	path           string
	root           string
	hasDeps        bool
	private        bool
	manager        string
	managerInvalid bool
	workspaces     []string
}

type workspaceDefinition struct {
	root     string
	patterns []string
	manager  string
}

type uvManifest struct {
	path       string
	root       string
	hasDeps    bool
	managed    bool
	workspaces []string
}

type cargoManifest struct {
	path        string
	root        string
	hasDeps     bool
	hasPackage  bool
	application bool
	workspaces  []string
}

type policyParseError struct {
	detector DetectorID
	path     string
	detail   string
}

type evaluationContext struct {
	sourceByPath     map[string]DependencySourceResult
	javascript       []javascriptManifest
	javascriptSpaces []workspaceDefinition
	uv               []uvManifest
	cargo            []cargoManifest
	parseErrors      []policyParseError
}

func evaluateChecks(sources []DependencySourceResult, discoveredPaths map[string]struct{}, checks []check) ([]CheckRun, []Finding) {
	if len(checks) == 0 {
		return []CheckRun{}, []Finding{}
	}
	ctx := buildEvaluationContext(sources, discoveredPaths)
	runs := make([]CheckRun, 0)
	findings := make([]Finding, 0)
	for _, configured := range checks {
		var checkRuns []CheckRun
		var checkFindings []Finding
		switch configured.EvaluatorType {
		case "npm-lockfile-missing":
			checkRuns, checkFindings = evaluateJavaScriptLockfile(ctx, configured, "npm", []string{"package-lock.json", "npm-shrinkwrap.json"})
		case "pnpm-lockfile-missing":
			checkRuns, checkFindings = evaluateJavaScriptLockfile(ctx, configured, "pnpm", []string{"pnpm-lock.yaml"})
		case "yarn-lockfile-missing":
			checkRuns, checkFindings = evaluateJavaScriptLockfile(ctx, configured, "yarn", []string{"yarn.lock"})
		case "uv-lockfile-missing":
			checkRuns, checkFindings = evaluateUVLockfile(ctx, configured)
		case "cargo-application-lockfile-missing":
			checkRuns, checkFindings = evaluateCargoLockfile(ctx, configured)
		}
		runs = append(runs, checkRuns...)
		findings = append(findings, checkFindings...)
	}
	slices.SortFunc(runs, compareCheckRun)
	slices.SortFunc(findings, compareFinding)
	return runs, findings
}

func buildEvaluationContext(sources []DependencySourceResult, discoveredPaths map[string]struct{}) evaluationContext {
	ctx := evaluationContext{
		sourceByPath: make(map[string]DependencySourceResult, len(sources)),
	}
	for _, source := range sources {
		ctx.sourceByPath[source.Path] = source
		switch source.Detector {
		case "js":
			if manifest, err := readJavaScriptManifest(source.content, source); err == nil {
				ctx.javascript = append(ctx.javascript, manifest)
			} else {
				ctx.parseErrors = append(ctx.parseErrors, policyParseError{detector: source.Detector, path: source.Path, detail: err.Error()})
			}
		case "js-pnpm-workspace":
			if workspace, err := readPNPMWorkspace(source.content, source.Path); err == nil {
				ctx.javascriptSpaces = append(ctx.javascriptSpaces, workspace)
			} else {
				ctx.parseErrors = append(ctx.parseErrors, policyParseError{detector: source.Detector, path: source.Path, detail: err.Error()})
			}
		case "js-yarnrc":
			root := path.Dir(source.Path)
			if root == "." {
				root = ""
			}
			ctx.javascriptSpaces = append(ctx.javascriptSpaces, workspaceDefinition{root: root, manager: "yarn"})
		case "python-pyproject":
			if manifest, err := readUVManifest(source.content, source); err == nil {
				ctx.uv = append(ctx.uv, manifest)
			} else {
				ctx.parseErrors = append(ctx.parseErrors, policyParseError{detector: source.Detector, path: source.Path, detail: err.Error()})
			}
		case "rust-cargo":
			if manifest, err := readCargoManifest(source.content, source, discoveredPaths); err == nil {
				ctx.cargo = append(ctx.cargo, manifest)
			} else {
				ctx.parseErrors = append(ctx.parseErrors, policyParseError{detector: source.Detector, path: source.Path, detail: err.Error()})
			}
		}
	}
	for _, manifest := range ctx.javascript {
		if len(manifest.workspaces) > 0 {
			ctx.javascriptSpaces = append(ctx.javascriptSpaces, workspaceDefinition{root: manifest.root, patterns: manifest.workspaces, manager: manifest.manager})
		}
	}
	slices.SortFunc(ctx.javascript, func(a, b javascriptManifest) int { return strings.Compare(a.path, b.path) })
	slices.SortFunc(ctx.javascriptSpaces, func(a, b workspaceDefinition) int { return strings.Compare(a.root, b.root) })
	slices.SortFunc(ctx.uv, func(a, b uvManifest) int { return strings.Compare(a.path, b.path) })
	slices.SortFunc(ctx.cargo, func(a, b cargoManifest) int { return strings.Compare(a.path, b.path) })
	return ctx
}

func readJavaScriptManifest(data []byte, source DependencySourceResult) (javascriptManifest, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return javascriptManifest{}, err
	}
	manifest := javascriptManifest{path: source.Path, root: path.Dir(source.Path), hasDeps: source.Analysis.Presence == PresencePresent}
	if manifest.root == "." {
		manifest.root = ""
	}
	_ = json.Unmarshal(raw["private"], &manifest.private)
	var packageManager string
	if value, ok := raw["packageManager"]; ok {
		if err := json.Unmarshal(value, &packageManager); err != nil {
			manifest.managerInvalid = true
		} else if packageManager != "" {
			manifest.manager = normalizeJavaScriptManager(packageManager)
			manifest.managerInvalid = manifest.manager == ""
		}
	}
	manifest.workspaces = decodeJavaScriptWorkspaces(raw["workspaces"])
	return manifest, nil
}

func normalizeJavaScriptManager(value string) string {
	name := value
	if index := strings.Index(value, "@"); index >= 0 {
		name = value[:index]
	}
	switch strings.ToLower(name) {
	case "npm", "pnpm", "yarn":
		return strings.ToLower(name)
	default:
		return ""
	}
}

func decodeJavaScriptWorkspaces(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var list []string
	if json.Unmarshal(raw, &list) == nil {
		return cleanPatterns(list)
	}
	var object struct {
		Packages []string `json:"packages"`
	}
	if json.Unmarshal(raw, &object) == nil {
		return cleanPatterns(object.Packages)
	}
	return nil
}

func readPNPMWorkspace(data []byte, sourcePath string) (workspaceDefinition, error) {
	var raw struct {
		Packages []string `yaml:"packages"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return workspaceDefinition{}, err
	}
	root := path.Dir(sourcePath)
	if root == "." {
		root = ""
	}
	return workspaceDefinition{root: root, patterns: cleanPatterns(raw.Packages), manager: "pnpm"}, nil
}

func readUVManifest(data []byte, source DependencySourceResult) (uvManifest, error) {
	var raw map[string]any
	if err := toml.Unmarshal(data, &raw); err != nil {
		return uvManifest{}, err
	}
	root := path.Dir(source.Path)
	if root == "." {
		root = ""
	}
	project, _ := raw["project"].(map[string]any)
	tool, _ := raw["tool"].(map[string]any)
	uv, uvManaged := tool["uv"].(map[string]any)
	manifest := uvManifest{path: source.Path, root: root, managed: uvManaged}
	manifest.hasDeps = nonEmptyValue(project["dependencies"]) || nonEmptyValue(project["optional-dependencies"]) || nonEmptyValue(raw["dependency-groups"])
	if workspace, ok := uv["workspace"].(map[string]any); ok {
		manifest.workspaces = stringsFromAny(workspace["members"])
	}
	return manifest, nil
}

func readCargoManifest(data []byte, source DependencySourceResult, discovered map[string]struct{}) (cargoManifest, error) {
	var raw map[string]any
	if err := toml.Unmarshal(data, &raw); err != nil {
		return cargoManifest{}, err
	}
	root := path.Dir(source.Path)
	if root == "." {
		root = ""
	}
	manifest := cargoManifest{path: source.Path, root: root}
	for _, key := range []string{"dependencies", "dev-dependencies", "build-dependencies"} {
		manifest.hasDeps = manifest.hasDeps || nonEmptyValue(raw[key])
	}
	manifest.application = nonEmptyValue(raw["bin"])
	if packageTable, ok := raw["package"].(map[string]any); ok {
		manifest.hasPackage = true
		manifest.application = manifest.application || nonEmptyValue(packageTable["default-run"])
	}
	mainPath := path.Join(root, "src/main.rs")
	if _, ok := discovered[mainPath]; ok {
		manifest.application = true
	}
	if workspace, ok := raw["workspace"].(map[string]any); ok {
		manifest.workspaces = stringsFromAny(workspace["members"])
		manifest.hasDeps = manifest.hasDeps || nonEmptyValue(workspace["dependencies"])
	}
	return manifest, nil
}

func evaluateJavaScriptLockfile(ctx evaluationContext, configured check, manager string, accepted []string) ([]CheckRun, []Finding) {
	type project struct {
		root     string
		manifest string
		hasDeps  bool
		private  bool
		managers map[string]struct{}
		invalid  bool
	}
	projects := make(map[string]*project)
	for _, manifest := range ctx.javascript {
		ownerRoot := javascriptWorkspaceOwner(manifest.root, ctx.javascriptSpaces)
		ownerManifest := manifest.path
		if candidate := path.Join(ownerRoot, "package.json"); ownerRoot == "" {
			candidate = "package.json"
			if _, ok := ctx.sourceByPath[candidate]; ok {
				ownerManifest = candidate
			}
		} else if _, ok := ctx.sourceByPath[candidate]; ok {
			ownerManifest = candidate
		}
		current := projects[ownerRoot]
		if current == nil {
			current = &project{root: ownerRoot, manifest: ownerManifest, managers: make(map[string]struct{})}
			projects[ownerRoot] = current
		}
		current.hasDeps = current.hasDeps || manifest.hasDeps
		current.private = current.private || manifest.private
		current.invalid = current.invalid || manifest.managerInvalid
		if manifest.manager != "" {
			current.managers[manifest.manager] = struct{}{}
		}
	}
	for _, workspace := range ctx.javascriptSpaces {
		if workspace.manager == "" {
			continue
		}
		if project := projects[workspace.root]; project != nil {
			project.managers[workspace.manager] = struct{}{}
		}
	}

	keys := make([]string, 0, len(projects))
	for key := range projects {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	runs := failedRunsForDetector(ctx, configured.ID, "js")
	if manager == "pnpm" {
		runs = append(runs, failedRunsForDetector(ctx, configured.ID, "js-pnpm-workspace")...)
	}
	findings := make([]Finding, 0)
	for _, key := range keys {
		project := projects[key]
		if !project.hasDeps {
			continue
		}
		lockManagers := javascriptLockManagers(project.root, ctx.sourceByPath)
		for value := range lockManagers {
			project.managers[value] = struct{}{}
		}
		if _, selected := project.managers[manager]; !selected {
			continue
		}
		subject := projectSubject(project.root, project.manifest)
		if project.invalid || len(project.managers) != 1 {
			runs = append(runs, skippedRun(configured.ID, subject, "ambiguous-package-manager", "conflicting or unsupported JavaScript package-manager evidence"))
			continue
		}
		if !project.private {
			runs = append(runs, skippedRun(configured.ID, subject, "project-role-unknown", "JavaScript application role could not be established"))
			continue
		}
		missing := true
		for _, filename := range accepted {
			if _, ok := ctx.sourceByPath[joinRoot(project.root, filename)]; ok {
				missing = false
				break
			}
		}
		runs = append(runs, completedRun(configured.ID, subject))
		if missing {
			expected := strings.Join(accepted, " or ")
			findings = append(findings, newMissingLockfileFinding(configured, subject, manager, expected))
		}
	}
	return runs, findings
}

func evaluateUVLockfile(ctx evaluationContext, configured check) ([]CheckRun, []Finding) {
	runs := failedRunsForDetector(ctx, configured.ID, "python-pyproject")
	findings := make([]Finding, 0)
	seen := make(map[string]struct{})
	workspaces := uvWorkspaces(ctx.uv)
	for _, manifest := range ctx.uv {
		if !manifest.hasDeps {
			continue
		}
		ownerRoot := tomlWorkspaceOwner(manifest.root, workspaces)
		managed := manifest.managed
		if ownerRoot != manifest.root {
			for _, candidate := range ctx.uv {
				if candidate.root == ownerRoot {
					managed = candidate.managed
					break
				}
			}
		}
		if !managed {
			continue
		}
		if _, ok := seen[ownerRoot]; ok {
			continue
		}
		seen[ownerRoot] = struct{}{}
		ownerManifest := manifest.path
		if candidate := joinRoot(ownerRoot, "pyproject.toml"); ctx.sourceByPath[candidate].Path != "" {
			ownerManifest = candidate
		}
		subject := projectSubject(ownerRoot, ownerManifest)
		runs = append(runs, completedRun(configured.ID, subject))
		if _, ok := ctx.sourceByPath[joinRoot(ownerRoot, "uv.lock")]; !ok {
			findings = append(findings, newMissingLockfileFinding(configured, subject, "uv", "uv.lock"))
		}
	}
	return runs, findings
}

func evaluateCargoLockfile(ctx evaluationContext, configured check) ([]CheckRun, []Finding) {
	runs := failedRunsForDetector(ctx, configured.ID, "rust-cargo")
	findings := make([]Finding, 0)
	seen := make(map[string]struct{})
	workspaces := cargoWorkspaces(ctx.cargo)
	for _, manifest := range ctx.cargo {
		if !manifest.hasDeps {
			continue
		}
		if !manifest.hasPackage && len(manifest.workspaces) > 0 {
			continue
		}
		ownerRoot := tomlWorkspaceOwner(manifest.root, workspaces)
		if !manifest.application {
			subject := projectSubject(manifest.root, manifest.path)
			runs = append(runs, skippedRun(configured.ID, subject, "project-role-unknown", "Cargo application role could not be established"))
			continue
		}
		if _, ok := seen[ownerRoot]; ok {
			continue
		}
		seen[ownerRoot] = struct{}{}
		ownerManifest := manifest.path
		if candidate := joinRoot(ownerRoot, "Cargo.toml"); ctx.sourceByPath[candidate].Path != "" {
			ownerManifest = candidate
		}
		subject := projectSubject(ownerRoot, ownerManifest)
		runs = append(runs, completedRun(configured.ID, subject))
		if _, ok := ctx.sourceByPath[joinRoot(ownerRoot, "Cargo.lock")]; !ok {
			findings = append(findings, newMissingLockfileFinding(configured, subject, "cargo", "Cargo.lock"))
		}
	}
	return runs, findings
}

func javascriptWorkspaceOwner(projectRoot string, workspaces []workspaceDefinition) string {
	owner := projectRoot
	bestLength := -1
	for _, workspace := range workspaces {
		if workspace.root == projectRoot || !isAncestorRoot(workspace.root, projectRoot) {
			continue
		}
		relative := strings.TrimPrefix(strings.TrimPrefix(projectRoot, workspace.root), "/")
		if workspaceMatches(relative, workspace.patterns) && len(workspace.root) > bestLength {
			owner = workspace.root
			bestLength = len(workspace.root)
		}
	}
	return owner
}

func tomlWorkspaceOwner(projectRoot string, workspaces []workspaceDefinition) string {
	return javascriptWorkspaceOwner(projectRoot, workspaces)
}

func uvWorkspaces(manifests []uvManifest) []workspaceDefinition {
	result := make([]workspaceDefinition, 0)
	for _, manifest := range manifests {
		if len(manifest.workspaces) > 0 {
			result = append(result, workspaceDefinition{root: manifest.root, patterns: manifest.workspaces, manager: "uv"})
		}
	}
	return result
}

func cargoWorkspaces(manifests []cargoManifest) []workspaceDefinition {
	result := make([]workspaceDefinition, 0)
	for _, manifest := range manifests {
		if len(manifest.workspaces) > 0 {
			result = append(result, workspaceDefinition{root: manifest.root, patterns: manifest.workspaces, manager: "cargo"})
		}
	}
	return result
}

func javascriptLockManagers(root string, sources map[string]DependencySourceResult) map[string]struct{} {
	result := make(map[string]struct{})
	for manager, names := range map[string][]string{
		"npm": {"package-lock.json", "npm-shrinkwrap.json"}, "pnpm": {"pnpm-lock.yaml"}, "yarn": {"yarn.lock"},
	} {
		for _, name := range names {
			if _, ok := sources[joinRoot(root, name)]; ok {
				result[manager] = struct{}{}
			}
		}
	}
	return result
}

func workspaceMatches(relative string, patterns []string) bool {
	matched := false
	for _, pattern := range patterns {
		exclude := strings.HasPrefix(pattern, "!")
		pattern = strings.TrimPrefix(pattern, "!")
		if pathGlobMatches(pattern, relative) {
			matched = !exclude
		}
	}
	return matched
}

func isAncestorRoot(ancestor, child string) bool {
	if ancestor == "" {
		return child != ""
	}
	return strings.HasPrefix(child, ancestor+"/")
}

func joinRoot(root, filename string) string {
	if root == "" {
		return filename
	}
	return path.Join(root, filename)
}

func projectSubject(root, manifest string) FindingSubject {
	key := root
	if key == "" {
		key = "."
	}
	return FindingSubject{Kind: "project", Key: key, Path: manifest}
}

func completedRun(id CheckID, subject FindingSubject) CheckRun {
	return CheckRun{CheckID: id, Subject: subject, Status: CheckCompleted}
}

func skippedRun(id CheckID, subject FindingSubject, reason, detail string) CheckRun {
	return CheckRun{CheckID: id, Subject: subject, Status: CheckSkipped, ReasonCode: reason, Detail: detail}
}

func failedRunsForDetector(ctx evaluationContext, id CheckID, detector DetectorID) []CheckRun {
	runs := make([]CheckRun, 0)
	for _, parseError := range ctx.parseErrors {
		if parseError.detector != detector {
			continue
		}
		root := path.Dir(parseError.path)
		if root == "." {
			root = ""
		}
		runs = append(runs, CheckRun{
			CheckID: id, Subject: projectSubject(root, parseError.path), Status: CheckFailed,
			ReasonCode: "source-analysis-failed", Detail: parseError.detail,
		})
	}
	return runs
}

func newMissingLockfileFinding(configured check, subject FindingSubject, manager, expected string) Finding {
	evidence := map[string]string{"manager": manager, "expected_lockfile": expected}
	return Finding{
		CheckID: configured.ID, Severity: configured.Severity, Summary: configured.Summary, Subject: subject,
		Locations: []FindingLocation{{Path: subject.Path}}, Evidence: evidence, Remediation: configured.Remediation,
		Fingerprint: findingFingerprint(configured.ID, subject, manager, expected),
	}
}

func findingFingerprint(id CheckID, subject FindingSubject, manager, expected string) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("1\x00%s\x00%s\x00%s\x00%s\x00%s", id, subject.Kind, subject.Key, manager, expected)))
	return fmt.Sprintf("%x", hash[:])
}

func compareCheckRun(a, b CheckRun) int {
	if value := strings.Compare(a.Subject.Path, b.Subject.Path); value != 0 {
		return value
	}
	return strings.Compare(string(a.CheckID), string(b.CheckID))
}

func compareFinding(a, b Finding) int {
	if value := strings.Compare(a.Subject.Path, b.Subject.Path); value != 0 {
		return value
	}
	return strings.Compare(string(a.CheckID), string(b.CheckID))
}

func cleanPatterns(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(strings.TrimPrefix(value, "./"))
		value = strings.TrimSuffix(value, "/")
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}

func stringsFromAny(value any) []string {
	list, ok := value.([]map[string]any)
	if ok {
		result := make([]string, 0, len(list))
		for _, item := range list {
			for _, nested := range item {
				if text, ok := nested.(string); ok {
					result = append(result, text)
				}
			}
		}
		return cleanPatterns(result)
	}
	values, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(values))
	for _, item := range values {
		if text, ok := item.(string); ok {
			result = append(result, text)
		}
	}
	return cleanPatterns(result)
}

func nonEmptyValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case string:
		return typed != ""
	case []any:
		return len(typed) > 0
	case []map[string]any:
		return len(typed) > 0
	case map[string]any:
		return len(typed) > 0
	default:
		return true
	}
}
