package analyze

import (
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

const (
	glueDefaultArguments        = "default_arguments"
	glueNonOverridableArguments = "non_overridable_arguments"
	glueLanguageArgument        = "--job-language"
	glueModulesArgument         = "--additional-python-modules"
	glueInstallerArgument       = "--python-modules-installer-option"
)

type terraformGluePythonConfig struct{}

type terraformGluePythonParser struct{}

type terraformArgumentState uint8

const (
	terraformArgumentMissing terraformArgumentState = iota
	terraformArgumentLiteral
	terraformArgumentUnknown
)

type terraformArgument struct {
	state terraformArgumentState
	value string
}

func newTerraformGluePythonParser(terraformGluePythonConfig) (sourceAnalyzer, error) {
	return terraformGluePythonParser{}, nil
}

func (terraformGluePythonParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL(content, path)
	if diags.HasErrors() {
		return sourceAnalyzerResult{}, fmt.Errorf("parse terraform file %q: %s", path, diags.Error())
	}
	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return sourceAnalyzerResult{}, fmt.Errorf("parse terraform file %q: unexpected body type %T", path, file.Body)
	}

	var groups []DependencyGroup
	var dependencies []DependencyReference
	var incomplete []string
	selected := false
	for _, block := range body.Blocks {
		if block.Type != "resource" || len(block.Labels) < 2 || block.Labels[0] != "aws_glue_job" {
			continue
		}
		name := block.Labels[1]
		location := fmt.Sprintf("line-%d-column-%d", block.TypeRange.Start.Line, block.TypeRange.Start.Column)
		context := fmt.Sprintf("Glue job %q at %s in %q", name, location, path)

		language := effectiveTerraformGlueArgument(block.Body, glueLanguageArgument)
		if language.state == terraformArgumentUnknown {
			selected = true
			incomplete = append(incomplete, context+" has a job language that cannot be evaluated statically")
			continue
		}
		if language.state == terraformArgumentLiteral && strings.EqualFold(strings.TrimSpace(language.value), "scala") {
			continue
		}
		if language.state == terraformArgumentLiteral && !strings.EqualFold(strings.TrimSpace(language.value), "python") {
			continue
		}
		selected = true

		modules := effectiveTerraformGlueArgument(block.Body, glueModulesArgument)
		switch modules.state {
		case terraformArgumentUnknown:
			incomplete = append(incomplete, context+" has an additional Python modules declaration that cannot be evaluated statically")
			continue
		case terraformArgumentMissing:
			groups = append(groups, DependencyGroup{Name: name, Location: location, State: GroupMissing})
			continue
		}

		installer := effectiveTerraformGlueArgument(block.Body, glueInstallerArgument)
		if installer.state == terraformArgumentUnknown {
			incomplete = append(incomplete, context+" has Python module installer options that cannot be evaluated statically")
			continue
		}
		if installer.state == terraformArgumentLiteral && strings.TrimSpace(installer.value) != "" {
			incomplete = append(incomplete, fmt.Sprintf("%s uses unsupported Python module installer options %q", context, installer.value))
			continue
		}

		group := DependencyGroup{Name: name, Location: location, State: GroupEmpty}
		rawDependencies, splitErr := splitTerraformGlueModules(modules.value)
		if splitErr != nil {
			incomplete = append(incomplete, fmt.Sprintf("%s has an invalid additional Python modules declaration: %v", context, splitErr))
			continue
		}
		invalid := false
		for _, raw := range rawDependencies {
			if strings.HasSuffix(strings.ToLower(strings.TrimSpace(raw)), ".whl") {
				incomplete = append(incomplete, fmt.Sprintf("%s has unsupported wheel dependency %q", context, raw))
				invalid = true
				continue
			}
			dependency, err := parsePythonRequirement(raw)
			if err != nil {
				incomplete = append(incomplete, fmt.Sprintf("%s has unsupported or invalid dependency %q: %v", context, raw, err))
				invalid = true
				continue
			}
			dependency.SourceGroup = name
			group.Dependencies = append(group.Dependencies, dependency)
		}
		if invalid {
			continue
		}
		if len(group.Dependencies) > 0 {
			group.State = GroupReady
			dependencies = append(dependencies, group.Dependencies...)
		}
		groups = append(groups, group)
	}

	if !selected {
		return sourceAnalyzerResult{}, nil
	}
	if len(incomplete) > 0 {
		analysis := failedAnalysis()
		severity := DiagnosticError
		if len(dependencies) > 0 {
			analysis = SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionPartial}
			severity = DiagnosticWarning
		}
		return sourceAnalyzerResult{
			Recognized: true, Analysis: analysis, Dependencies: dependencies, Groups: groups,
			Diagnostics: diagnosticsFromMessages(severity, "terraform-glue-incomplete", incomplete),
		}, nil
	}
	return sourceAnalyzerResult{Recognized: true, Analysis: completeAnalysis(dependencies), Dependencies: dependencies, Groups: groups}, nil
}

func splitTerraformGlueModules(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	var parts []string
	start, brackets := 0, 0
	var quote rune
	for index, character := range value {
		switch {
		case quote != 0:
			if character == quote {
				quote = 0
			}
		case character == '\'' || character == '"':
			quote = character
		case character == '[':
			brackets++
		case character == ']':
			if brackets > 0 {
				brackets--
			}
		case character == ',' && brackets == 0:
			part := strings.TrimSpace(value[start:index])
			if part == "" {
				return nil, fmt.Errorf("empty entry")
			}
			if beginsPythonVersionConstraint(part) && len(parts) > 0 {
				parts[len(parts)-1] += "," + part
			} else {
				parts = append(parts, part)
			}
			start = index + 1
		}
	}
	part := strings.TrimSpace(value[start:])
	if part == "" {
		return nil, fmt.Errorf("empty entry")
	}
	if beginsPythonVersionConstraint(part) && len(parts) > 0 {
		parts[len(parts)-1] += "," + part
	} else {
		parts = append(parts, part)
	}
	return parts, nil
}

func beginsPythonVersionConstraint(value string) bool {
	for _, operator := range []string{"===", "==", "!=", "~=", "<=", ">=", "<", ">"} {
		if strings.HasPrefix(value, operator) {
			return true
		}
	}
	return false
}

func effectiveTerraformGlueArgument(body *hclsyntax.Body, key string) terraformArgument {
	nonOverridable := terraformGlueArgument(body, glueNonOverridableArguments, key)
	if nonOverridable.state != terraformArgumentMissing {
		return nonOverridable
	}
	return terraformGlueArgument(body, glueDefaultArguments, key)
}

func terraformGlueArgument(body *hclsyntax.Body, attributeName, key string) terraformArgument {
	attribute, ok := body.Attributes[attributeName]
	if !ok {
		return terraformArgument{state: terraformArgumentMissing}
	}
	expression := unwrapTerraformParentheses(attribute.Expr)
	object, ok := expression.(*hclsyntax.ObjectConsExpr)
	if !ok {
		return terraformArgument{state: terraformArgumentUnknown}
	}
	for _, item := range object.Items {
		itemKey, ok := terraformLiteralString(item.KeyExpr)
		if !ok {
			return terraformArgument{state: terraformArgumentUnknown}
		}
		if itemKey != key {
			continue
		}
		value, ok := terraformLiteralString(item.ValueExpr)
		if !ok {
			return terraformArgument{state: terraformArgumentUnknown}
		}
		return terraformArgument{state: terraformArgumentLiteral, value: value}
	}
	return terraformArgument{state: terraformArgumentMissing}
}

func unwrapTerraformParentheses(expression hclsyntax.Expression) hclsyntax.Expression {
	for {
		parentheses, ok := expression.(*hclsyntax.ParenthesesExpr)
		if !ok {
			return expression
		}
		expression = parentheses.Expression
	}
}

func terraformLiteralString(expression hclsyntax.Expression) (string, bool) {
	value, diags := expression.Value(nil)
	if diags.HasErrors() || !value.IsKnown() || value.IsNull() || value.Type() != cty.String {
		return "", false
	}
	value, _ = value.Unmark()
	return value.AsString(), true
}
