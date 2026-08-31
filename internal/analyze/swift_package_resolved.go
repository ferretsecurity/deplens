package analyze

import (
	"encoding/json"
	"fmt"
	"strings"
)

type swiftPackageResolvedParser struct{}

type swiftPackageResolvedFile struct {
	Version int                         `json:"version"`
	Pins    []swiftPackageResolvedPin   `json:"pins"`
	Object  *swiftPackageResolvedObject `json:"object"`
}

type swiftPackageResolvedObject struct {
	Pins []swiftPackageResolvedPin `json:"pins"`
}

type swiftPackageResolvedPin struct {
	Identity      string                    `json:"identity"`
	Package       string                    `json:"package"`
	Location      string                    `json:"location"`
	RepositoryURL string                    `json:"repositoryURL"`
	State         swiftPackageResolvedState `json:"state"`
}

type swiftPackageResolvedState struct {
	Branch   string `json:"branch"`
	Revision string `json:"revision"`
	Version  string `json:"version"`
}

func newSwiftPackageResolvedParser(swiftPackageResolvedMatcherConfig) (sourceAnalyzer, error) {
	return swiftPackageResolvedParser{}, nil
}

func (swiftPackageResolvedParser) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var file swiftPackageResolvedFile
	if err := json.Unmarshal(content, &file); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse Swift Package.resolved lockfile %q: %w", path, err)
	}

	pins, ok := swiftPackageResolvedPins(file)
	if !ok {
		return sourceAnalyzerResult{}, nil
	}

	dependencies := make([]DependencyReference, 0, len(pins))
	incomplete := make([]string, 0)
	for index, pin := range pins {
		dependency, message := swiftPackageResolvedDependency(pin)
		if message != "" {
			incomplete = append(incomplete, fmt.Sprintf("pins[%d]: %s", index, message))
			continue
		}
		dependencies = append(dependencies, dependency)
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func swiftPackageResolvedPins(file swiftPackageResolvedFile) ([]swiftPackageResolvedPin, bool) {
	switch file.Version {
	case 1:
		if file.Object == nil {
			return nil, false
		}
		return file.Object.Pins, true
	case 2, 3:
		return file.Pins, true
	default:
		return nil, false
	}
}

func swiftPackageResolvedDependency(pin swiftPackageResolvedPin) (DependencyReference, string) {
	name := strings.TrimSpace(pin.Identity)
	if name == "" {
		name = strings.TrimSpace(pin.Package)
	}
	if name == "" {
		return DependencyReference{}, "package identity is required"
	}

	location := strings.TrimSpace(pin.Location)
	if location == "" {
		location = strings.TrimSpace(pin.RepositoryURL)
	}
	if location == "" {
		return DependencyReference{}, "source location is required"
	}

	revision := strings.TrimSpace(pin.State.Revision)
	if revision == "" {
		return DependencyReference{}, "resolved revision is required"
	}
	version := strings.TrimSpace(pin.State.Version)
	if version == "" {
		version = revision
	}

	attributes := map[string]string{
		"source_url":      location,
		"source_ref":      revision,
		"source_ref_kind": "commit",
	}
	if branch := strings.TrimSpace(pin.State.Branch); branch != "" {
		attributes["source_branch"] = branch
	}

	return DependencyReference{
		Raw:          name + "@" + version,
		Name:         name,
		Version:      version,
		SourceGroup:  "pins",
		OriginKind:   OriginGit,
		Relationship: RelationshipInconclusive,
		Scope:        ScopeRuntime,
		Attributes:   attributes,
	}, ""
}
