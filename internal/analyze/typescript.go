package analyze

import (
	"fmt"
	"strconv"
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
	tstypescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
)

const typescriptSplitComma = "comma"

type typescriptMatcherConfig struct {
	CDKConstruct *typescriptCDKConstructConfig `yaml:"cdk_construct"`
}

type typescriptCDKConstructConfig struct {
	Module             string                            `yaml:"module"`
	Construct          string                            `yaml:"construct"`
	PropsArgumentIndex *int                              `yaml:"props_argument_index"`
	Within             []string                          `yaml:"within"`
	Conditions         []typescriptObjectConditionConfig `yaml:"conditions"`
	Extract            *typescriptExtractConfig          `yaml:"extract"`
}

type typescriptObjectConditionConfig struct {
	Key     string  `yaml:"key"`
	Equals  *string `yaml:"equals"`
	Present bool    `yaml:"present"`
}

type typescriptExtractConfig struct {
	Key   string `yaml:"key"`
	Split string `yaml:"split"`
}

type typescriptObjectCondition struct {
	key     string
	equals  *string
	present bool
}

type typescriptExtract struct {
	key   string
	split string
}

type typeScriptImportTable struct {
	namespaces map[string]struct{}
	named      map[string]struct{}
}

type typescriptCDKConstructMatcher struct {
	module             string
	construct          string
	propsArgumentIndex int
	within             []string
	conditions         []typescriptObjectCondition
	extract            *typescriptExtract
}

func newTypeScriptMatcher(raw typescriptMatcherConfig) (manifestParser, error) {
	if raw.CDKConstruct == nil {
		return nil, fmt.Errorf("typescript.cdk_construct: required")
	}

	cfg := raw.CDKConstruct
	if cfg.Module == "" {
		return nil, fmt.Errorf("typescript.cdk_construct.module: required")
	}
	if cfg.Construct == "" {
		return nil, fmt.Errorf("typescript.cdk_construct.construct: required")
	}
	if cfg.PropsArgumentIndex == nil {
		return nil, fmt.Errorf("typescript.cdk_construct.props_argument_index: required")
	}
	if *cfg.PropsArgumentIndex < 0 {
		return nil, fmt.Errorf("typescript.cdk_construct.props_argument_index: must be >= 0")
	}
	if len(cfg.Conditions) == 0 {
		return nil, fmt.Errorf("typescript.cdk_construct.conditions: must contain at least one entry")
	}

	within := make([]string, 0, len(cfg.Within))
	for idx, segment := range cfg.Within {
		if segment == "" {
			return nil, fmt.Errorf("typescript.cdk_construct.within[%d]: required", idx)
		}
		within = append(within, segment)
	}

	conditions := make([]typescriptObjectCondition, 0, len(cfg.Conditions))
	for idx, cond := range cfg.Conditions {
		if cond.Key == "" {
			return nil, fmt.Errorf("typescript.cdk_construct.conditions[%d].key: required", idx)
		}
		if cond.Equals == nil && !cond.Present {
			return nil, fmt.Errorf("typescript.cdk_construct.conditions[%d]: one of equals or present=true is required", idx)
		}
		conditions = append(conditions, typescriptObjectCondition{
			key:     cond.Key,
			equals:  cond.Equals,
			present: cond.Present,
		})
	}

	var extract *typescriptExtract
	if cfg.Extract != nil {
		if cfg.Extract.Key == "" {
			return nil, fmt.Errorf("typescript.cdk_construct.extract.key: required")
		}
		if cfg.Extract.Split != typescriptSplitComma {
			return nil, fmt.Errorf("typescript.cdk_construct.extract.split: unsupported value %q", cfg.Extract.Split)
		}
		extract = &typescriptExtract{
			key:   cfg.Extract.Key,
			split: cfg.Extract.Split,
		}
	}

	return typescriptCDKConstructMatcher{
		module:             cfg.Module,
		construct:          cfg.Construct,
		propsArgumentIndex: *cfg.PropsArgumentIndex,
		within:             within,
		conditions:         conditions,
		extract:            extract,
	}, nil
}

func (m typescriptCDKConstructMatcher) Match(path string, content []byte) ([]Dependency, *bool, bool, error) {
	parser := sitter.NewParser()
	defer parser.Close()

	if err := parser.SetLanguage(sitter.NewLanguage(tstypescript.LanguageTypescript())); err != nil {
		return nil, nil, false, fmt.Errorf("configure typescript parser for %q: %w", path, err)
	}

	tree := parser.Parse(content, nil)
	defer tree.Close()

	root := tree.RootNode()
	imports := collectTypeScriptImports(root, content, m.module, m.construct)
	if len(imports.namespaces) == 0 && len(imports.named) == 0 {
		return nil, nil, false, nil
	}

	var (
		dependencies []string
		matched      bool
	)

	walkNamedNodes(root, func(node *sitter.Node) bool {
		if node.Kind() != "new_expression" {
			return true
		}

		deps, ok := m.matchNewExpression(root, node, content, imports)
		if !ok {
			return true
		}

		dependencies = deps
		matched = true
		return false
	})

	if !matched {
		return nil, nil, false, nil
	}
	if len(dependencies) == 0 {
		return dependenciesFromStrings(dependencies), nil, true, nil
	}
	return dependenciesFromStrings(dependencies), boolPtr(true), true, nil
}

func (m typescriptCDKConstructMatcher) matchNewExpression(root *sitter.Node, node *sitter.Node, content []byte, imports typeScriptImportTable) ([]string, bool) {
	constructor := node.ChildByFieldName("constructor")
	if !m.matchesConstructor(constructor, content, imports) {
		return nil, false
	}

	argsNode := node.ChildByFieldName("arguments")
	if argsNode == nil {
		return nil, false
	}

	args := namedChildren(argsNode)
	if m.propsArgumentIndex >= len(args) {
		return nil, false
	}

	objectNode, ok := resolveTypeScriptObjectNode(root, args[m.propsArgumentIndex], content)
	if !ok {
		return nil, false
	}

	for _, segment := range m.within {
		next, ok := objectPropertyValue(objectNode, content, segment)
		if !ok || next.Kind() != "object" {
			return nil, false
		}
		objectNode = next
	}

	for _, cond := range m.conditions {
		valueNode, ok := objectPropertyValue(objectNode, content, cond.key)
		if !ok {
			return nil, false
		}
		if cond.present {
			continue
		}

		value, ok := resolveTypeScriptStringValue(root, valueNode, content)
		if !ok || value != *cond.equals {
			return nil, false
		}
	}

	if m.extract == nil {
		return nil, true
	}

	valueNode, ok := objectPropertyValue(objectNode, content, m.extract.key)
	if !ok {
		return nil, false
	}

	value, ok := resolveTypeScriptStringValue(root, valueNode, content)
	if !ok {
		return nil, true
	}

	dependencies := splitExtractedValue(value, m.extract.split)
	if len(dependencies) == 0 {
		return nil, true
	}

	return dependencies, true
}

func resolveTypeScriptObjectNode(root *sitter.Node, node *sitter.Node, content []byte) (*sitter.Node, bool) {
	if node == nil {
		return nil, false
	}
	if node.Kind() == "object" {
		return node, true
	}
	if node.Kind() != "identifier" {
		return nil, false
	}

	name := identifierText(node, content)
	if name == "" {
		return nil, false
	}

	var resolved *sitter.Node
	walkNamedNodes(root, func(candidate *sitter.Node) bool {
		if candidate.Kind() != "variable_declarator" || candidate.EndByte() >= node.StartByte() {
			return true
		}

		nameNode := candidate.ChildByFieldName("name")
		if identifierText(nameNode, content) != name {
			return true
		}

		valueNode := candidate.ChildByFieldName("value")
		if valueNode == nil || valueNode.Kind() != "object" {
			return true
		}

		if resolved == nil || resolved.EndByte() < candidate.EndByte() {
			resolved = valueNode
		}
		return true
	})

	if resolved == nil {
		return nil, false
	}
	return resolved, true
}

func resolveTypeScriptStringValue(root *sitter.Node, node *sitter.Node, content []byte) (string, bool) {
	resolvedNode, ok := resolveTypeScriptValueNode(root, node, content, "string")
	if !ok {
		return "", false
	}
	return stringLiteralValue(resolvedNode, content)
}

func resolveTypeScriptValueNode(root *sitter.Node, node *sitter.Node, content []byte, expectedKind string) (*sitter.Node, bool) {
	if node == nil {
		return nil, false
	}
	if node.Kind() == expectedKind {
		return node, true
	}
	if node.Kind() != "identifier" {
		return nil, false
	}

	name := identifierText(node, content)
	if name == "" {
		return nil, false
	}

	var resolved *sitter.Node
	walkNamedNodes(root, func(candidate *sitter.Node) bool {
		if candidate.Kind() != "variable_declarator" || candidate.EndByte() >= node.StartByte() {
			return true
		}

		nameNode := candidate.ChildByFieldName("name")
		if identifierText(nameNode, content) != name {
			return true
		}

		valueNode := candidate.ChildByFieldName("value")
		if valueNode == nil || valueNode.Kind() != expectedKind {
			return true
		}

		if resolved == nil || resolved.EndByte() < candidate.EndByte() {
			resolved = valueNode
		}
		return true
	})

	if resolved == nil {
		return nil, false
	}
	return resolved, true
}

func (m typescriptCDKConstructMatcher) matchesConstructor(node *sitter.Node, content []byte, imports typeScriptImportTable) bool {
	if node == nil {
		return false
	}

	switch node.Kind() {
	case "identifier":
		identifier := identifierText(node, content)
		_, ok := imports.named[identifier]
		return ok
	case "member_expression":
		objectNode := node.ChildByFieldName("object")
		propertyNode := node.ChildByFieldName("property")
		if objectNode == nil || propertyNode == nil || objectNode.Kind() != "identifier" {
			return false
		}
		propertyName := propertyNameText(propertyNode, content)
		if propertyName != m.construct {
			return false
		}
		_, ok := imports.namespaces[identifierText(objectNode, content)]
		return ok
	default:
		return false
	}
}

func collectTypeScriptImports(root *sitter.Node, content []byte, module string, construct string) typeScriptImportTable {
	imports := typeScriptImportTable{
		namespaces: make(map[string]struct{}),
		named:      make(map[string]struct{}),
	}

	walkNamedNodes(root, func(node *sitter.Node) bool {
		if node.Kind() != "import_statement" {
			return true
		}

		sourceNode := node.ChildByFieldName("source")
		source, ok := stringLiteralValue(sourceNode, content)
		if !ok || source != module {
			return false
		}

		for _, child := range namedChildren(node) {
			if child.Kind() != "import_clause" {
				continue
			}
			for _, clauseChild := range namedChildren(child) {
				switch clauseChild.Kind() {
				case "namespace_import":
					namespaceIdentifier := clauseChild.NamedChild(0)
					if namespaceIdentifier != nil {
						imports.namespaces[identifierText(namespaceIdentifier, content)] = struct{}{}
					}
				case "named_imports":
					for _, specifier := range namedChildren(clauseChild) {
						if specifier.Kind() != "import_specifier" {
							continue
						}
						nameNode := specifier.ChildByFieldName("name")
						if identifierText(nameNode, content) != construct {
							continue
						}
						aliasNode := specifier.ChildByFieldName("alias")
						if aliasNode != nil {
							imports.named[identifierText(aliasNode, content)] = struct{}{}
							continue
						}
						imports.named[identifierText(nameNode, content)] = struct{}{}
					}
				}
			}
		}

		return false
	})

	return imports
}

func splitExtractedValue(value string, mode string) []string {
	if mode != typescriptSplitComma {
		return nil
	}

	parts := strings.Split(value, ",")
	dependencies := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		dependencies = append(dependencies, part)
	}
	return dependencies
}

func objectPropertyValue(objectNode *sitter.Node, content []byte, key string) (*sitter.Node, bool) {
	for _, child := range namedChildren(objectNode) {
		if child.Kind() != "pair" {
			continue
		}
		keyNode := child.ChildByFieldName("key")
		if objectKeyText(keyNode, content) != key {
			continue
		}
		valueNode := child.ChildByFieldName("value")
		if valueNode == nil {
			return nil, false
		}
		return valueNode, true
	}
	return nil, false
}

func objectKeyText(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}
	switch node.Kind() {
	case "property_identifier":
		return propertyNameText(node, content)
	case "string":
		value, ok := stringLiteralValue(node, content)
		if ok {
			return value
		}
	}
	return ""
}

func stringLiteralValue(node *sitter.Node, content []byte) (string, bool) {
	if node == nil || node.Kind() != "string" {
		return "", false
	}
	raw := nodeText(node, content)
	value, err := strconv.Unquote(raw)
	if err != nil {
		return "", false
	}
	return value, true
}

func propertyNameText(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}
	return nodeText(node, content)
}

func identifierText(node *sitter.Node, content []byte) string {
	if node == nil || node.Kind() != "identifier" {
		return ""
	}
	return nodeText(node, content)
}

func nodeText(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}
	return string(content[node.StartByte():node.EndByte()])
}

func namedChildren(node *sitter.Node) []*sitter.Node {
	if node == nil {
		return nil
	}

	children := make([]*sitter.Node, 0, node.NamedChildCount())
	for idx := uint(0); idx < node.NamedChildCount(); idx++ {
		children = append(children, node.NamedChild(idx))
	}
	return children
}

func walkNamedNodes(node *sitter.Node, visit func(*sitter.Node) bool) {
	if node == nil {
		return
	}
	if !visit(node) {
		return
	}
	for idx := uint(0); idx < node.NamedChildCount(); idx++ {
		walkNamedNodes(node.NamedChild(idx), visit)
	}
}
