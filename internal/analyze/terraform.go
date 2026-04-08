package analyze

import (
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

const terraformResourceParser = "terraform_resource"

type terraformConditionConfig struct {
	Path    string  `yaml:"path"`
	Equals  *string `yaml:"equals"`
	Present bool    `yaml:"present"`
}

type terraformResourceParserMatcher struct {
	resourceType string
	conditions   []terraformCondition
}

type terraformCondition struct {
	path    []string
	equals  *string
	present bool
}

type terraformMatcherConfig struct {
	ResourceType string                     `yaml:"resource_type"`
	Conditions   []terraformConditionConfig `yaml:"conditions"`
}

func compileManifestParser(raw ruleConfig) (manifestParser, error) {
	parserCount := 0
	if raw.BannerRegex != "" {
		parserCount++
	}
	if raw.Terraform != nil {
		parserCount++
	}
	if raw.INI != nil {
		parserCount++
	}
	if raw.TypeScript != nil {
		parserCount++
	}
	if raw.Python != nil {
		parserCount++
	}
	if raw.YAML != nil {
		parserCount++
	}
	if raw.TOML != nil {
		parserCount++
	}
	if raw.JSON != nil {
		parserCount++
	}
	if raw.HTML != nil {
		parserCount++
	}
	if parserCount > 1 {
		return nil, fmt.Errorf("exactly one parser type may be configured")
	}
	if raw.BannerRegex != "" {
		return newBannerRegexParser(raw.BannerRegex)
	}
	if raw.Terraform != nil {
		return newTerraformResourceParser(*raw.Terraform)
	}
	if raw.INI != nil {
		return newINIQueryParser(*raw.INI)
	}
	if raw.TypeScript != nil {
		return newTypeScriptMatcher(*raw.TypeScript)
	}
	if raw.Python != nil {
		return newPythonMatcher(*raw.Python)
	}
	if raw.YAML != nil {
		return newYAMLQueryParser(*raw.YAML)
	}
	if raw.TOML != nil {
		return newTOMLQueryParser(*raw.TOML)
	}
	if raw.JSON != nil {
		return newJSONMatcher(*raw.JSON)
	}
	if raw.HTML != nil {
		return newHTMLMatcher(*raw.HTML)
	}
	return nil, nil
}

func newTerraformResourceParser(raw terraformMatcherConfig) (manifestParser, error) {
	if raw.ResourceType == "" {
		return nil, fmt.Errorf("terraform.resource_type: required")
	}
	if len(raw.Conditions) == 0 {
		return nil, fmt.Errorf("terraform.conditions: must contain at least one entry")
	}

	conditions := make([]terraformCondition, 0, len(raw.Conditions))
	for idx, cond := range raw.Conditions {
		if cond.Path == "" {
			return nil, fmt.Errorf("terraform.conditions[%d].path: required", idx)
		}
		if cond.Equals == nil && !cond.Present {
			return nil, fmt.Errorf("terraform.conditions[%d]: one of equals or present=true is required", idx)
		}

		conditions = append(conditions, terraformCondition{
			path:    strings.Split(cond.Path, "."),
			equals:  cond.Equals,
			present: cond.Present,
		})
	}

	return terraformResourceParserMatcher{
		resourceType: raw.ResourceType,
		conditions:   conditions,
	}, nil
}

func (m terraformResourceParserMatcher) Match(path string, content []byte) ([]Dependency, *bool, bool, error) {
	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL(content, path)
	if diags.HasErrors() {
		return nil, nil, false, fmt.Errorf("parse terraform file %q: %s", path, diags.Error())
	}

	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return nil, nil, false, fmt.Errorf("parse terraform file %q: unexpected body type %T", path, file.Body)
	}

	for _, block := range body.Blocks {
		if block.Type != "resource" || len(block.Labels) == 0 || block.Labels[0] != m.resourceType {
			continue
		}
		matched, err := m.matchBlock(block.Body)
		if err != nil {
			return nil, nil, false, fmt.Errorf("match terraform resource %q in %q: %w", m.resourceType, path, err)
		}
		if matched {
			return nil, nil, true, nil
		}
	}

	return nil, nil, false, nil
}

func (m terraformResourceParserMatcher) matchBlock(body *hclsyntax.Body) (bool, error) {
	for _, cond := range m.conditions {
		matched, err := matchesTerraformCondition(body, cond)
		if err != nil {
			return false, err
		}
		if !matched {
			return false, nil
		}
	}
	return true, nil
}

func matchesTerraformCondition(body *hclsyntax.Body, cond terraformCondition) (bool, error) {
	if len(cond.path) == 0 {
		return false, nil
	}

	attr, ok := body.Attributes[cond.path[0]]
	if !ok {
		return false, nil
	}

	value, diags := attr.Expr.Value(nil)
	if diags.HasErrors() {
		return false, nil
	}
	value, _ = value.Unmark()

	for _, segment := range cond.path[1:] {
		if !value.IsKnown() || value.IsNull() {
			return false, nil
		}
		switch {
		case value.Type().IsObjectType():
			if !value.Type().HasAttribute(segment) {
				return false, nil
			}
			value = value.GetAttr(segment)
			value, _ = value.Unmark()
		case value.Type().IsMapType():
			key := cty.StringVal(segment)
			has := value.HasIndex(key)
			if !has.IsKnown() || has.False() {
				return false, nil
			}
			value = value.Index(key)
			value, _ = value.Unmark()
		default:
			return false, nil
		}
	}

	if cond.present {
		return value.IsKnown() && !value.IsNull(), nil
	}
	if cond.equals == nil {
		return false, nil
	}
	if !value.IsKnown() || value.IsNull() || value.Type() != cty.String {
		return false, nil
	}

	return value.AsString() == *cond.equals, nil
}
