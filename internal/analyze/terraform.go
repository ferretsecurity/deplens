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

func (m terraformResourceParserMatcher) Match(path string, content []byte) (manifestParserResult, error) {
	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL(content, path)
	if diags.HasErrors() {
		return manifestParserResult{}, fmt.Errorf("parse terraform file %q: %s", path, diags.Error())
	}

	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return manifestParserResult{}, fmt.Errorf("parse terraform file %q: unexpected body type %T", path, file.Body)
	}

	for _, block := range body.Blocks {
		if block.Type != "resource" || len(block.Labels) == 0 || block.Labels[0] != m.resourceType {
			continue
		}
		matched, err := m.matchBlock(block.Body)
		if err != nil {
			return manifestParserResult{}, fmt.Errorf("match terraform resource %q in %q: %w", m.resourceType, path, err)
		}
		if matched {
			return manifestParserResult{Matched: true}, nil
		}
	}

	return manifestParserResult{}, nil
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
