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

type manifestRule struct {
	Type   ManifestType
	Regexp *regexp.Regexp
	Parser manifestParser
}

type Ruleset struct {
	rules          []manifestRule
	supportedTypes []ManifestType
}

type rulesFile struct {
	Rules []ruleConfig `yaml:"rules"`
}

type ruleConfig struct {
	Name          string                  `yaml:"name"`
	FilenameRegex string                  `yaml:"filename-regex"`
	Terraform     *terraformMatcherConfig `yaml:"terraform"`
}

type manifestParser interface {
	Match(path string, content []byte) (bool, error)
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
		if rawRule.FilenameRegex == "" {
			return Ruleset{}, fmt.Errorf("%s: %s.filename-regex: required", source, fieldPath)
		}

		compiled, err := regexp.Compile(rawRule.FilenameRegex)
		if err != nil {
			return Ruleset{}, fmt.Errorf("%s: %s.filename-regex: compile %q: %w", source, fieldPath, rawRule.FilenameRegex, err)
		}

		parser, err := compileManifestParser(rawRule)
		if err != nil {
			return Ruleset{}, fmt.Errorf("%s: %s: %w", source, fieldPath, err)
		}

		rules = append(rules, manifestRule{
			Type:   ManifestType(rawRule.Name),
			Regexp: compiled,
			Parser: parser,
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

func (r Ruleset) DetectManifest(name string) (ManifestType, bool) {
	for _, rule := range r.rules {
		if rule.Parser != nil {
			continue
		}
		if rule.Regexp.MatchString(name) {
			return rule.Type, true
		}
	}
	return "", false
}

func (r Ruleset) DetectManifestFile(path string, name string) (ManifestType, bool, error) {
	var content []byte
	contentLoaded := false

	for _, rule := range r.rules {
		if !rule.Regexp.MatchString(name) {
			continue
		}
		if rule.Parser == nil {
			return rule.Type, true, nil
		}
		if !contentLoaded {
			data, err := os.ReadFile(path)
			if err != nil {
				return "", false, fmt.Errorf("read candidate file %q: %w", path, err)
			}
			content = data
			contentLoaded = true
		}
		ok, err := rule.Parser.Match(path, content)
		if err != nil {
			return "", false, err
		}
		if ok {
			return rule.Type, true, nil
		}
	}
	return "", false, nil
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
