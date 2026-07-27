package analyze

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strings"
)

type dotnetProjectParser struct{}
type dotnetCentralPackagesParser struct{}
type dotnetPackagesConfigParser struct{}

type dotnetXMLNode struct {
	name       string
	attributes map[string]string
	text       string
	children   []*dotnetXMLNode
}

var msbuildPropertyExpression = regexp.MustCompile(`\$\(([^)]+)\)`)

func newDotnetProjectParser(dotnetProjectMatcherConfig) (sourceAnalyzer, error) {
	return dotnetProjectParser{}, nil
}

func newDotnetCentralPackagesParser(dotnetCentralPackagesMatcherConfig) (sourceAnalyzer, error) {
	return dotnetCentralPackagesParser{}, nil
}

func newDotnetPackagesConfigParser(dotnetPackagesConfigMatcherConfig) (sourceAnalyzer, error) {
	return dotnetPackagesConfigParser{}, nil
}

func (dotnetProjectParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	root, err := parseDotnetXML(path, content)
	if err != nil {
		return sourceAnalyzerResult{}, err
	}
	if root.name != "Project" {
		return sourceAnalyzerResult{}, nil
	}
	properties := dotnetProperties(root)
	dependencies, incomplete := dotnetMSBuildItems(root, "PackageReference", RelationshipDirect, properties)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func (dotnetCentralPackagesParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	root, err := parseDotnetXML(path, content)
	if err != nil {
		return sourceAnalyzerResult{}, err
	}
	if root.name != "Project" {
		return sourceAnalyzerResult{}, nil
	}
	properties := dotnetProperties(root)
	dependencies, incomplete := dotnetMSBuildItems(root, "PackageVersion", RelationshipInconclusive, properties)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func (dotnetPackagesConfigParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	root, err := parseDotnetXML(path, content)
	if err != nil {
		return sourceAnalyzerResult{}, err
	}
	if root.name != "packages" {
		return sourceAnalyzerResult{}, nil
	}
	dependencies := make([]DependencyReference, 0)
	incomplete := make([]string, 0)
	for _, child := range root.children {
		if child.name != "package" {
			continue
		}
		name := strings.TrimSpace(child.attributes["id"])
		version := strings.TrimSpace(child.attributes["version"])
		if name == "" {
			incomplete = append(incomplete, "packages.config contains a package without an id")
			continue
		}
		dependency := DependencyReference{
			Raw:          name,
			Name:         name,
			SourceGroup:  "package",
			Relationship: RelationshipDirect,
			Scope:        ScopeRuntime,
		}
		if version != "" {
			dependency.Raw += "@" + version
			dependency.Version = version
		}
		if strings.EqualFold(child.attributes["developmentDependency"], "true") {
			dependency.Scope = ScopeDevelopment
		}
		if framework := strings.TrimSpace(child.attributes["targetFramework"]); framework != "" {
			dependency.Attributes = map[string]string{"target_framework": framework}
		}
		dependencies = append(dependencies, dependency)
	}
	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func parseDotnetXML(path string, content []byte) (*dotnetXMLNode, error) {
	decoder := xml.NewDecoder(bytes.NewReader(content))
	var root *dotnetXMLNode
	stack := make([]*dotnetXMLNode, 0)
	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				if root == nil {
					return nil, fmt.Errorf("parse .NET XML %q: empty document", path)
				}
				return root, nil
			}
			return nil, fmt.Errorf("parse .NET XML %q: %w", path, err)
		}
		switch typed := token.(type) {
		case xml.StartElement:
			node := &dotnetXMLNode{name: typed.Name.Local, attributes: make(map[string]string)}
			for _, attribute := range typed.Attr {
				node.attributes[attribute.Name.Local] = attribute.Value
			}
			if len(stack) == 0 {
				if root != nil {
					return nil, fmt.Errorf("parse .NET XML %q: multiple root elements", path)
				}
				root = node
			} else {
				parent := stack[len(stack)-1]
				parent.children = append(parent.children, node)
			}
			stack = append(stack, node)
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].text += string(typed)
			}
		case xml.EndElement:
			if len(stack) == 0 {
				return nil, fmt.Errorf("parse .NET XML %q: unexpected closing element", path)
			}
			stack = stack[:len(stack)-1]
		}
	}
}

func dotnetProperties(root *dotnetXMLNode) map[string]string {
	properties := make(map[string]string)
	for _, group := range root.children {
		if group.name != "PropertyGroup" || strings.TrimSpace(group.attributes["Condition"]) != "" {
			continue
		}
		for _, property := range group.children {
			if strings.TrimSpace(property.attributes["Condition"]) != "" || len(property.children) > 0 {
				continue
			}
			properties[property.name] = strings.TrimSpace(property.text)
		}
	}
	return properties
}

func dotnetMSBuildItems(root *dotnetXMLNode, itemName string, defaultRelationship Relationship, properties map[string]string) ([]DependencyReference, []string) {
	dependencies := make([]DependencyReference, 0)
	incomplete := make([]string, 0)
	for _, group := range root.children {
		if group.name != "ItemGroup" {
			continue
		}
		for _, item := range group.children {
			if item.name != itemName {
				continue
			}
			include := strings.TrimSpace(item.attributes["Include"])
			update := strings.TrimSpace(item.attributes["Update"])
			name := include
			relationship := defaultRelationship
			operation := "Include"
			if name == "" {
				name = update
				relationship = RelationshipInconclusive
				operation = "Update"
			}
			if name == "" {
				incomplete = append(incomplete, fmt.Sprintf("%s contains an item without Include or Update", itemName))
				continue
			}

			rawVersion := dotnetItemValue(item, "Version")
			dependency := DependencyReference{
				Raw:          name,
				Name:         name,
				SourceGroup:  itemName,
				Relationship: relationship,
				Scope:        dotnetItemScope(item),
			}
			if rawVersion != "" {
				dependency.Raw += "@" + rawVersion
				resolvedVersion, ok := resolveMSBuildValue(rawVersion, properties)
				if ok {
					dependency.VersionConstraint = resolvedVersion
				} else {
					dependency.VersionConstraint = rawVersion
					incomplete = append(incomplete, fmt.Sprintf("%s %s uses unresolved version expression %q", itemName, name, rawVersion))
				}
			}
			attributes := make(map[string]string)
			if operation == "Update" {
				attributes["item_operation"] = operation
			}
			if condition := combineMSBuildConditions(group.attributes["Condition"], item.attributes["Condition"]); condition != "" {
				attributes["condition"] = condition
			}
			for _, field := range []string{"IncludeAssets", "PrivateAssets", "ExcludeAssets"} {
				if value := dotnetItemValue(item, field); value != "" {
					attributes[msbuildAttributeName(field)] = value
				}
			}
			if len(attributes) > 0 {
				dependency.Attributes = attributes
			}
			dependencies = append(dependencies, dependency)
		}
	}
	sortDependencyReferences(dependencies)
	return dependencies, incomplete
}

func dotnetItemValue(node *dotnetXMLNode, field string) string {
	if value := strings.TrimSpace(node.attributes[field]); value != "" {
		return value
	}
	for _, child := range node.children {
		if child.name == field {
			return strings.TrimSpace(child.text)
		}
	}
	return ""
}

func dotnetItemScope(item *dotnetXMLNode) DependencyScope {
	if strings.EqualFold(dotnetItemValue(item, "DevelopmentDependency"), "true") {
		return ScopeDevelopment
	}
	includeAssets := splitMSBuildAssets(dotnetItemValue(item, "IncludeAssets"))
	if len(includeAssets) > 0 {
		buildOnly := true
		for _, asset := range includeAssets {
			switch asset {
			case "build", "buildtransitive", "analyzers":
			default:
				buildOnly = false
			}
		}
		if buildOnly {
			return ScopeBuild
		}
	}
	return ScopeRuntime
}

func splitMSBuildAssets(value string) []string {
	fields := strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return r == ';' || r == ',' || r == ' '
	})
	result := fields[:0]
	for _, field := range fields {
		if field != "" {
			result = append(result, field)
		}
	}
	return result
}

func resolveMSBuildValue(value string, properties map[string]string) (string, bool) {
	current := strings.TrimSpace(value)
	for range 16 {
		unresolved := false
		changed := false
		next := msbuildPropertyExpression.ReplaceAllStringFunc(current, func(expression string) string {
			match := msbuildPropertyExpression.FindStringSubmatch(expression)
			replacement, ok := properties[match[1]]
			if !ok || replacement == "" {
				unresolved = true
				return expression
			}
			changed = true
			return replacement
		})
		current = next
		if unresolved {
			return current, false
		}
		if !changed {
			return current, !msbuildPropertyExpression.MatchString(current)
		}
	}
	return current, false
}

func combineMSBuildConditions(group, item string) string {
	values := make([]string, 0, 2)
	for _, value := range []string{group, item} {
		if value = strings.TrimSpace(value); value != "" {
			values = append(values, value)
		}
	}
	return strings.Join(values, " && ")
}

func msbuildAttributeName(value string) string {
	var words []string
	start := 0
	for index := 1; index < len(value); index++ {
		if value[index] >= 'A' && value[index] <= 'Z' {
			words = append(words, strings.ToLower(value[start:index]))
			start = index
		}
	}
	words = append(words, strings.ToLower(value[start:]))
	return strings.Join(words, "_")
}
