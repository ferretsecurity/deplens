package analyze

import (
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

type terraformLockMatcherConfig struct{}

type terraformLockParser struct{}

func newTerraformLockParser(terraformLockMatcherConfig) (sourceAnalyzer, error) {
	return terraformLockParser{}, nil
}

func (terraformLockParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL(content, path)
	if diags.HasErrors() {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Terraform lockfile %q: %s", path, diags.Error())
	}

	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Terraform lockfile %q: unexpected body type %T", path, file.Body)
	}

	dependencies := make([]DependencyReference, 0)
	incomplete := make([]string, 0)
	for index, block := range body.Blocks {
		if block.Type != "provider" {
			continue
		}

		dependency, message := terraformLockDependency(block, index)
		if message != "" {
			incomplete = append(incomplete, message)
			continue
		}
		dependencies = append(dependencies, dependency)
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func terraformLockDependency(block *hclsyntax.Block, index int) (DependencyReference, string) {
	if len(block.Labels) != 1 || strings.TrimSpace(block.Labels[0]) == "" {
		return DependencyReference{}, fmt.Sprintf("provider[%d]: exactly one provider address is required", index)
	}
	name := strings.TrimSpace(block.Labels[0])
	version, ok := terraformLockAttributeString(block.Body, "version")
	if !ok || version == "" {
		return DependencyReference{}, fmt.Sprintf("provider.%s.version: a static string is required", name)
	}

	dependency := DependencyReference{
		PackageType:  "generic",
		Raw:          name + "@" + version,
		Name:         name,
		Version:      version,
		SourceGroup:  "providers",
		OriginKind:   OriginRegistry,
		Relationship: RelationshipDirect,
		Scope:        ScopeRuntime,
	}
	if constraint, exists := terraformLockAttributeString(block.Body, "constraints"); exists && constraint != "" {
		dependency.VersionConstraint = constraint
	}
	return dependency, ""
}

func terraformLockAttributeString(body *hclsyntax.Body, name string) (string, bool) {
	attribute, ok := body.Attributes[name]
	if !ok {
		return "", false
	}
	value, diags := attribute.Expr.Value(nil)
	if diags.HasErrors() || !value.IsKnown() || value.IsNull() || value.Type() != cty.String {
		return "", false
	}
	return strings.TrimSpace(value.AsString()), true
}
