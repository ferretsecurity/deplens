package analyze

import (
	_ "embed"
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

func boolPtr(v bool) *bool {
	return &v
}

//go:embed default_rules.yaml
var defaultRulesYAML []byte

type ManifestType string

type manifestRule struct {
	Type           ManifestType
	FilenameRegexp *regexp.Regexp
	PathGlob       string
	Parser         manifestParser
}

type Ruleset struct {
	rules          []manifestRule
	supportedTypes []ManifestType
}

type rulesFile struct {
	Rules []ruleConfig `yaml:"rules"`
}

type ruleConfig struct {
	Name           string                       `yaml:"name"`
	FilenameRegex  string                       `yaml:"filename-regex"`
	PathGlob       string                       `yaml:"path-glob"`
	BannerRegex    string                       `yaml:"banner-regex"`
	Terraform      *terraformMatcherConfig      `yaml:"terraform"`
	INI            *iniMatcherConfig            `yaml:"ini"`
	TypeScript     *typescriptMatcherConfig     `yaml:"typescript"`
	Python         *pythonMatcherConfig         `yaml:"python"`
	PyRequirements *pyRequirementsMatcherConfig `yaml:"py-requirements"`
	PoetryLock     *poetryLockMatcherConfig     `yaml:"poetry-lock"`
	UVLock         *uvLockMatcherConfig         `yaml:"uv-lock"`
	GoMod          *goModMatcherConfig          `yaml:"go-mod"`
	PackageLock    *packageLockMatcherConfig    `yaml:"package-lock"`
	ComposerLock   *composerLockMatcherConfig   `yaml:"composer-lock"`
	YAML           *yamlMatcherConfig           `yaml:"yaml"`
	TOML           *tomlMatcherConfig           `yaml:"toml"`
	JSON           *jsonMatcherConfig           `yaml:"json"`
	XML            *xmlMatcherConfig            `yaml:"xml"`
	HTML           *htmlMatcherConfig           `yaml:"html"`
}

type uvLockMatcherConfig struct{}

type poetryLockMatcherConfig struct{}

type packageLockMatcherConfig struct{}

type composerLockMatcherConfig struct{}

type manifestParser interface {
	Match(path string, content []byte) (manifestParserResult, error)
}

type manifestParserResult struct {
	Dependencies    []Dependency
	HasDependencies *bool
	Warnings        []string
	Matched         bool
}

func LoadDefaultRules() (Ruleset, error) {
	return loadRules("embedded default rules", defaultRulesYAML)
}

func LoadRulesFile(path string) (Ruleset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Ruleset{}, fmt.Errorf("read rules file %q: %w", path, err)
	}
	return loadRules(path, data)
}

func loadRules(source string, data []byte) (Ruleset, error) {
	var raw rulesFile
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return Ruleset{}, fmt.Errorf("parse rules from %s: %w", source, err)
	}
	if len(raw.Rules) == 0 {
		return Ruleset{}, fmt.Errorf("%s: rules: must contain at least one rule", source)
	}

	rules := make([]manifestRule, 0, len(raw.Rules))
	for ruleIdx, rawRule := range raw.Rules {
		fieldPath := fmt.Sprintf("rules[%d]", ruleIdx)
		if rawRule.Name == "" {
			return Ruleset{}, fmt.Errorf("%s: %s.name: required", source, fieldPath)
		}
		if rawRule.FilenameRegex == "" && rawRule.PathGlob == "" {
			return Ruleset{}, fmt.Errorf("%s: %s: at least one selector is required", source, fieldPath)
		}

		var compiled *regexp.Regexp
		var err error
		if rawRule.FilenameRegex != "" {
			compiled, err = regexp.Compile(rawRule.FilenameRegex)
			if err != nil {
				return Ruleset{}, fmt.Errorf("%s: %s.filename-regex: compile %q: %w", source, fieldPath, rawRule.FilenameRegex, err)
			}
		}
		if rawRule.PathGlob != "" {
			if err := validatePathGlob(rawRule.PathGlob); err != nil {
				return Ruleset{}, fmt.Errorf("%s: %s.path-glob: compile %q: %w", source, fieldPath, rawRule.PathGlob, err)
			}
		}

		parser, err := compileManifestParser(rawRule)
		if err != nil {
			return Ruleset{}, fmt.Errorf("%s: %s: %w", source, fieldPath, err)
		}

		rules = append(rules, manifestRule{
			Type:           ManifestType(rawRule.Name),
			FilenameRegexp: compiled,
			PathGlob:       rawRule.PathGlob,
			Parser:         parser,
		})
	}

	return Ruleset{
		rules:          rules,
		supportedTypes: supportedTypesFromRules(rules),
	}, nil
}

func (r Ruleset) SupportedManifestTypes() []ManifestType {
	return append([]ManifestType(nil), r.supportedTypes...)
}

func (r Ruleset) DetectSelectorOnlyManifest(name string) (ManifestType, bool) {
	for _, rule := range r.rules {
		if rule.PathGlob != "" {
			continue
		}
		if rule.Parser != nil {
			continue
		}
		if rule.matches(name, "") {
			return rule.Type, true
		}
	}
	return "", false
}

func (r Ruleset) DetectManifestFile(path string, name string) (ManifestType, []Dependency, *bool, []string, bool, error) {
	return r.detectManifestFile(path, name, "")
}

func (r Ruleset) DetectManifestFileAtRelativePath(path string, name string, relPath string) (ManifestType, []Dependency, *bool, []string, bool, error) {
	return r.detectManifestFile(path, name, normalizeRelativePath(relPath))
}

func (r Ruleset) detectManifestFile(path string, name string, relPath string) (ManifestType, []Dependency, *bool, []string, bool, error) {
	var content []byte
	contentLoaded := false

	for _, rule := range r.rules {
		if !rule.matches(name, relPath) {
			continue
		}
		if rule.Parser == nil {
			return rule.Type, nil, nil, nil, true, nil
		}
		if path == "" {
			continue
		}
		if !contentLoaded {
			data, err := os.ReadFile(path)
			if err != nil {
				return "", nil, nil, nil, false, fmt.Errorf("read candidate file %q: %w", path, err)
			}
			content = data
			contentLoaded = true
		}
		result, err := rule.Parser.Match(path, content)
		if err != nil {
			return "", nil, nil, nil, false, err
		}
		if result.Matched {
			return rule.Type, result.Dependencies, result.HasDependencies, result.Warnings, true, nil
		}
	}
	return "", nil, nil, nil, false, nil
}

func (r manifestRule) matches(name string, relPath string) bool {
	if r.FilenameRegexp != nil && !r.FilenameRegexp.MatchString(name) {
		return false
	}
	if r.PathGlob != "" && !pathGlobMatches(r.PathGlob, relPath) {
		return false
	}
	return true
}

func pathGlobMatches(pattern string, relPath string) bool {
	patternSegments := strings.Split(pattern, "/")
	pathSegments := strings.Split(relPath, "/")
	return matchPathGlobSegments(patternSegments, pathSegments)
}

func matchPathGlobSegments(patternSegments []string, pathSegments []string) bool {
	if len(patternSegments) == 0 {
		return len(pathSegments) == 0
	}

	if patternSegments[0] == "**" {
		if matchPathGlobSegments(patternSegments[1:], pathSegments) {
			return true
		}
		for i := 0; i < len(pathSegments); i++ {
			if matchPathGlobSegments(patternSegments[1:], pathSegments[i+1:]) {
				return true
			}
		}
		return false
	}

	if len(pathSegments) == 0 {
		return false
	}

	ok, err := path.Match(patternSegments[0], pathSegments[0])
	if err != nil || !ok {
		return false
	}

	return matchPathGlobSegments(patternSegments[1:], pathSegments[1:])
}

func validatePathGlob(pattern string) error {
	for _, segment := range strings.Split(pattern, "/") {
		if segment == "" {
			return fmt.Errorf("empty path segment")
		}
		if segment == "**" {
			continue
		}
		if strings.Contains(segment, "**") {
			return fmt.Errorf("invalid recursive wildcard segment %q", segment)
		}
		if _, err := path.Match(segment, ""); err != nil {
			return err
		}
	}
	return nil
}

func supportedTypesFromRules(rules []manifestRule) []ManifestType {
	types := make([]ManifestType, 0)
	seen := make(map[ManifestType]struct{})
	for _, rule := range rules {
		if _, ok := seen[rule.Type]; ok {
			continue
		}
		seen[rule.Type] = struct{}{}
		types = append(types, rule.Type)
	}
	return types
}
