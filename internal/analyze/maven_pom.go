package analyze

import (
	"encoding/xml"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

type mavenPOMParser struct{}

type mavenPOM struct {
	XMLName              xml.Name                  `xml:"project"`
	ModelVersion         string                    `xml:"modelVersion"`
	GroupID              string                    `xml:"groupId"`
	ArtifactID           string                    `xml:"artifactId"`
	Version              string                    `xml:"version"`
	Parent               mavenParent               `xml:"parent"`
	Properties           mavenProperties           `xml:"properties"`
	Dependencies         []mavenDependency         `xml:"dependencies>dependency"`
	DependencyManagement mavenDependencyManagement `xml:"dependencyManagement"`
}

type mavenParent struct {
	GroupID string `xml:"groupId"`
	Version string `xml:"version"`
}

type mavenDependencyManagement struct {
	Dependencies []mavenDependency `xml:"dependencies>dependency"`
}

type mavenDependency struct {
	GroupID    string `xml:"groupId"`
	ArtifactID string `xml:"artifactId"`
	Version    string `xml:"version"`
	Scope      string `xml:"scope"`
	Type       string `xml:"type"`
	Classifier string `xml:"classifier"`
	Optional   string `xml:"optional"`
}

type mavenProperties map[string]string

func (p *mavenProperties) UnmarshalXML(decoder *xml.Decoder, start xml.StartElement) error {
	values := make(map[string]string)
	for {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch typed := token.(type) {
		case xml.StartElement:
			var value string
			if err := decoder.DecodeElement(&value, &typed); err != nil {
				return err
			}
			values[typed.Name.Local] = strings.TrimSpace(value)
		case xml.EndElement:
			if typed.Name == start.Name {
				*p = values
				return nil
			}
		}
	}
}

var mavenPropertyExpression = regexp.MustCompile(`\$\{([^}]+)\}`)

func newMavenPOMParser(mavenPOMMatcherConfig) (sourceAnalyzer, error) {
	return mavenPOMParser{}, nil
}

func (mavenPOMParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var pom mavenPOM
	if err := xml.Unmarshal(content, &pom); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Maven POM %q: %w", path, err)
	}
	if pom.XMLName.Local != "project" {
		return sourceAnalyzerResult{}, nil
	}

	properties := mavenPOMProperties(pom)
	dependencies := make([]DependencyReference, 0, len(pom.Dependencies)+len(pom.DependencyManagement.Dependencies))
	incomplete := make([]string, 0)
	dependencies, incomplete = appendMavenDependencies(dependencies, incomplete, pom.Dependencies, "dependencies", RelationshipDirect, properties)
	dependencies, incomplete = appendMavenDependencies(dependencies, incomplete, pom.DependencyManagement.Dependencies, "dependencyManagement", RelationshipInconclusive, properties)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func mavenPOMProperties(pom mavenPOM) map[string]string {
	properties := make(map[string]string, len(pom.Properties)+8)
	for key, value := range pom.Properties {
		properties[key] = value
	}
	groupID := strings.TrimSpace(pom.GroupID)
	if groupID == "" {
		groupID = strings.TrimSpace(pom.Parent.GroupID)
	}
	version := strings.TrimSpace(pom.Version)
	if version == "" {
		version = strings.TrimSpace(pom.Parent.Version)
	}
	for _, key := range []string{"project.groupId", "pom.groupId"} {
		properties[key] = groupID
	}
	for _, key := range []string{"project.artifactId", "pom.artifactId"} {
		properties[key] = strings.TrimSpace(pom.ArtifactID)
	}
	for _, key := range []string{"project.version", "pom.version"} {
		properties[key] = version
	}
	return properties
}

func appendMavenDependencies(dependencies []DependencyReference, incomplete []string, values []mavenDependency, group string, relationship Relationship, properties map[string]string) ([]DependencyReference, []string) {
	ordered := slices.Clone(values)
	slices.SortFunc(ordered, func(a, b mavenDependency) int {
		return strings.Compare(strings.TrimSpace(a.GroupID)+":"+strings.TrimSpace(a.ArtifactID), strings.TrimSpace(b.GroupID)+":"+strings.TrimSpace(b.ArtifactID))
	})
	for _, value := range ordered {
		rawGroup := strings.TrimSpace(value.GroupID)
		rawArtifact := strings.TrimSpace(value.ArtifactID)
		rawVersion := strings.TrimSpace(value.Version)
		if rawGroup == "" || rawArtifact == "" {
			incomplete = append(incomplete, fmt.Sprintf("%s contains a dependency without both groupId and artifactId", group))
			continue
		}
		resolvedGroup, groupOK := resolveMavenValue(rawGroup, properties)
		resolvedArtifact, artifactOK := resolveMavenValue(rawArtifact, properties)
		name := resolvedGroup + ":" + resolvedArtifact
		if !groupOK || !artifactOK {
			name = rawGroup + ":" + rawArtifact
			incomplete = append(incomplete, fmt.Sprintf("%s %s uses an unresolved coordinate expression", group, name))
		}
		dependency := DependencyReference{
			Raw:          rawGroup + ":" + rawArtifact,
			Name:         name,
			SourceGroup:  group,
			Relationship: relationship,
			Scope:        mavenDependencyScope(value),
		}
		if rawVersion != "" {
			dependency.Raw += ":" + rawVersion
			resolvedVersion, ok := resolveMavenValue(rawVersion, properties)
			if ok {
				dependency.VersionConstraint = normalizeMavenManifestConstraint(resolvedVersion)
			} else {
				dependency.VersionConstraint = rawVersion
				incomplete = append(incomplete, fmt.Sprintf("%s %s uses unresolved version expression %q", group, name, rawVersion))
			}
		}
		attributes := make(map[string]string)
		if scope := strings.TrimSpace(value.Scope); scope != "" && scope != "compile" {
			attributes["maven_scope"] = scope
		}
		if packageType := strings.TrimSpace(value.Type); packageType != "" && packageType != "jar" {
			attributes["type"] = packageType
		}
		if classifier := strings.TrimSpace(value.Classifier); classifier != "" {
			attributes["classifier"] = classifier
		}
		if len(attributes) > 0 {
			dependency.Attributes = attributes
		}
		dependencies = append(dependencies, dependency)
	}
	return dependencies, incomplete
}

func resolveMavenValue(value string, properties map[string]string) (string, bool) {
	current := strings.TrimSpace(value)
	for range 16 {
		unresolved := false
		changed := false
		next := mavenPropertyExpression.ReplaceAllStringFunc(current, func(expression string) string {
			match := mavenPropertyExpression.FindStringSubmatch(expression)
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
			return current, !mavenPropertyExpression.MatchString(current)
		}
	}
	return current, false
}

func normalizeMavenManifestConstraint(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "[") || strings.HasPrefix(value, "(") {
		return value
	}
	return "[" + value + "]"
}

func mavenDependencyScope(value mavenDependency) DependencyScope {
	if strings.EqualFold(strings.TrimSpace(value.Optional), "true") {
		return ScopeOptional
	}
	switch strings.ToLower(strings.TrimSpace(value.Scope)) {
	case "test":
		return ScopeTest
	case "provided", "system", "import":
		return ScopeBuild
	default:
		return ScopeRuntime
	}
}
