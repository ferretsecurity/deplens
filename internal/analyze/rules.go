package analyze

import (
	_ "embed"
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

//go:embed default_rules.yaml
var defaultRulesYAML []byte

type ManifestType string

const (
	RequirementsTXT ManifestType = "requirements.txt"
	UVLock          ManifestType = "uv.lock"
	PackageJSON     ManifestType = "package.json"
	YarnLock        ManifestType = "yarn.lock"
	PomXML          ManifestType = "pom.xml"
)

type Rule struct {
	Name     string
	Patterns []manifestPattern
}

type manifestPattern struct {
	Type   ManifestType
	Regexp *regexp.Regexp
}

type Ruleset struct {
	rules          []Rule
	supportedTypes []ManifestType
}

type rulesFile struct {
	Rules []ruleConfig `yaml:"rules"`
}

type ruleConfig struct {
	Name     string          `yaml:"name"`
	Patterns []patternConfig `yaml:"patterns"`
}

type patternConfig struct {
	Type  string `yaml:"type"`
	Regex string `yaml:"regex"`
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

	rules := make([]Rule, 0, len(raw.Rules))
	for ruleIdx, rawRule := range raw.Rules {
		fieldPath := fmt.Sprintf("rules[%d]", ruleIdx)
		if rawRule.Name == "" {
			return Ruleset{}, fmt.Errorf("%s: %s.name: required", source, fieldPath)
		}
		if len(rawRule.Patterns) == 0 {
			return Ruleset{}, fmt.Errorf("%s: %s.patterns: must contain at least one pattern", source, fieldPath)
		}

		rule := Rule{
			Name:     rawRule.Name,
			Patterns: make([]manifestPattern, 0, len(rawRule.Patterns)),
		}
		for patternIdx, rawPattern := range rawRule.Patterns {
			patternPath := fmt.Sprintf("%s.patterns[%d]", fieldPath, patternIdx)
			if rawPattern.Type == "" {
				return Ruleset{}, fmt.Errorf("%s: %s.type: required", source, patternPath)
			}
			if rawPattern.Regex == "" {
				return Ruleset{}, fmt.Errorf("%s: %s.regex: required", source, patternPath)
			}

			compiled, err := regexp.Compile(rawPattern.Regex)
			if err != nil {
				return Ruleset{}, fmt.Errorf("%s: %s.regex: compile %q: %w", source, patternPath, rawPattern.Regex, err)
			}

			rule.Patterns = append(rule.Patterns, manifestPattern{
				Type:   ManifestType(rawPattern.Type),
				Regexp: compiled,
			})
		}
		rules = append(rules, rule)
	}

	return Ruleset{
		rules:          rules,
		supportedTypes: supportedTypesFromRules(rules),
	}, nil
}

func (r Ruleset) SupportedManifestTypes() []ManifestType {
	return append([]ManifestType(nil), r.supportedTypes...)
}

func (r Ruleset) DetectManifest(name string) (ManifestType, bool) {
	for _, rule := range r.rules {
		for _, pattern := range rule.Patterns {
			if pattern.Regexp.MatchString(name) {
				return pattern.Type, true
			}
		}
	}
	return "", false
}

func supportedTypesFromRules(rules []Rule) []ManifestType {
	types := make([]ManifestType, 0)
	seen := make(map[ManifestType]struct{})
	for _, rule := range rules {
		for _, pattern := range rule.Patterns {
			if _, ok := seen[pattern.Type]; ok {
				continue
			}
			seen[pattern.Type] = struct{}{}
			types = append(types, pattern.Type)
		}
	}
	return types
}
