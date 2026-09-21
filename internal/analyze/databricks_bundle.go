package analyze

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type databricksBundleConfig struct{}
type databricksBundleAnalyzer struct{}

func newDatabricksBundleAnalyzer(databricksBundleConfig) (sourceAnalyzer, error) {
	return databricksBundleAnalyzer{}, nil
}

func (databricksBundleAnalyzer) Analyze(path string, content []byte) (sourceAnalyzerResult, error) {
	var root any
	if err := yaml.Unmarshal(content, &root); err != nil {
		return sourceAnalyzerResult{}, fmt.Errorf("parse yaml file %q: %w", path, err)
	}
	rootMap, ok := asStringMap(root)
	if !ok {
		return sourceAnalyzerResult{}, nil
	}
	var groups []DependencyGroup
	var dependencies []DependencyReference
	var structural []string
	recognized := false
	inspect := func(scope any, segments []any) {
		scopeMap, ok := asStringMap(scope)
		if !ok {
			return
		}
		resources, exists := scopeMap["resources"]
		if !exists {
			return
		}
		resourcesMap, ok := asStringMap(resources)
		if !ok {
			structural = append(structural, jqPath(appendCopy(segments, "resources"))+": must be a mapping")
			return
		}
		jobs, exists := resourcesMap["jobs"]
		if !exists {
			return
		}
		jobsMap, ok := asStringMap(jobs)
		if !ok {
			structural = append(structural, jqPath(appendCopy(segments, "resources", "jobs"))+": must be a mapping")
			return
		}
		for _, jobName := range sortedStringKeys(jobsMap) {
			job, ok := asStringMap(jobsMap[jobName])
			if !ok {
				continue
			}
			rawTasks, exists := job["tasks"]
			if !exists {
				continue
			}
			recognized = true
			tasks, ok := rawTasks.([]any)
			if !ok {
				location := jqPath(appendCopy(segments, "resources", "jobs", jobName, "tasks"))
				message := location + ": must be a list"
				structural = append(structural, message)
				groups = append(groups, malformedDatabricksGroups(location, message)...)
				continue
			}
			for i, rawTask := range tasks {
				location := jqPath(appendCopy(segments, "resources", "jobs", jobName, "tasks", i))
				task, ok := asStringMap(rawTask)
				if !ok {
					message := location + ": task must be a mapping"
					structural = append(structural, message)
					groups = append(groups, malformedDatabricksGroups(location, message)...)
					continue
				}
				name, ok := task["task_key"].(string)
				if !ok || name == "" {
					structural = append(structural, location+".task_key: must be a non-empty string")
				}
				py := DependencyGroup{Name: name, Location: location, Format: GenerationPythonRequirements, State: GroupMissing}
				mv := DependencyGroup{Name: name, Location: location, Format: GenerationMavenPOM, State: GroupMissing}
				rawLibraries, exists := task["libraries"]
				if !exists || rawLibraries == nil {
					groups = append(groups, py, mv)
					continue
				}
				libraries, ok := rawLibraries.([]any)
				if !ok {
					message := location + ".libraries: must be a list"
					structural = append(structural, message)
					groupDiagnostic(&py, message)
					groupDiagnostic(&mv, message)
					groups = append(groups, py, mv)
					continue
				}
				if len(libraries) == 0 {
					py.State, mv.State = GroupEmpty, GroupEmpty
				}
				pyDeclared, mvDeclared := false, false
				for j, rawLibrary := range libraries {
					loc := fmt.Sprintf("%s.libraries[%d]", location, j)
					library, ok := asStringMap(rawLibrary)
					if !ok {
						structural = append(structural, loc+": library must be a mapping")
						continue
					}
					if raw, exists := library["pypi"]; exists {
						pyDeclared = true
						parseDatabricksPyPI(raw, loc, &py)
					}
					if _, exists := library["whl"]; exists {
						pyDeclared = true
						groupDiagnostic(&py, loc+".whl: wheel declarations are unsupported for generation")
					}
					if raw, exists := library["maven"]; exists {
						mvDeclared = true
						parseDatabricksMaven(raw, loc, &mv)
					}
					for _, kind := range []string{"jar", "egg"} {
						if _, exists := library[kind]; exists {
							mvDeclared = true
							groupDiagnostic(&mv, loc+"."+kind+": JAR/path declarations are unsupported for Maven generation")
						}
					}
				}
				if pyDeclared {
					py.State = GroupReady
				}
				if mvDeclared {
					mv.State = GroupReady
				}
				dependencies = append(dependencies, py.Dependencies...)
				dependencies = append(dependencies, mv.Dependencies...)
				groups = append(groups, py, mv)
			}
		}
	}
	inspect(rootMap, nil)
	if rawTargets, exists := rootMap["targets"]; exists {
		if targets, ok := asStringMap(rawTargets); ok {
			for _, name := range sortedStringKeys(targets) {
				inspect(targets[name], []any{"targets", name})
			}
		} else {
			structural = append(structural, ".targets: must be a mapping")
		}
	}
	if !recognized {
		return sourceAnalyzerResult{}, nil
	}
	presence := PresenceAbsent
	if len(dependencies) > 0 {
		presence = PresencePresent
	}
	analysis := SourceAnalysis{Presence: presence, Extraction: ExtractionComplete}
	var diagnostics []Diagnostic
	var declarationMessages []string
	for _, group := range groups {
		for _, diagnostic := range group.Diagnostics {
			declarationMessages = append(declarationMessages, diagnostic.Message)
		}
	}
	if len(declarationMessages) > 0 {
		analysis.Extraction = ExtractionPartial
		diagnostics = []Diagnostic{{Severity: DiagnosticWarning, Code: "databricks-bundle-incomplete", Message: strings.Join(declarationMessages, "; ")}}
	}
	if len(structural) > 0 {
		analysis.Extraction = ExtractionFailed
		diagnostics = []Diagnostic{{Severity: DiagnosticError, Code: "databricks-bundle-incomplete", Message: strings.Join(structural, "; ")}}
		for i := range groups {
			groups[i].Diagnostics = append(groups[i].Diagnostics, diagnostics[0])
		}
	}
	return sourceAnalyzerResult{Recognized: true, Analysis: analysis, Dependencies: dependencies, Groups: groups, Diagnostics: diagnostics}, nil
}

func appendCopy(base []any, values ...any) []any {
	out := append([]any{}, base...)
	return append(out, values...)
}
func malformedDatabricksGroups(location, message string) []DependencyGroup {
	py := DependencyGroup{Location: location, Format: GenerationPythonRequirements, State: GroupMissing}
	mv := DependencyGroup{Location: location, Format: GenerationMavenPOM, State: GroupMissing}
	groupDiagnostic(&py, message)
	groupDiagnostic(&mv, message)
	return []DependencyGroup{py, mv}
}
func groupDiagnostic(group *DependencyGroup, message string) {
	group.Diagnostics = append(group.Diagnostics, Diagnostic{Severity: DiagnosticError, Code: "databricks-bundle-incomplete", Message: message})
}
func parseDatabricksPyPI(raw any, location string, group *DependencyGroup) {
	pypi, ok := asStringMap(raw)
	if !ok {
		groupDiagnostic(group, location+".pypi: must be a mapping")
		return
	}
	spec, ok := pypi["package"].(string)
	if !ok || strings.TrimSpace(spec) == "" {
		groupDiagnostic(group, location+".pypi.package: must be a non-empty string")
		return
	}
	if repo, exists := pypi["repo"]; exists && repo != nil {
		groupDiagnostic(group, location+".pypi.repo: custom repositories are unsupported for generation")
		return
	}
	dep, err := parsePythonRequirement(spec)
	if err != nil {
		groupDiagnostic(group, fmt.Sprintf("%s.pypi.package %q: %v", location, spec, err))
		return
	}
	dep.SourceGroup = group.Name
	group.Dependencies = append(group.Dependencies, dep)
}
func parseDatabricksMaven(raw any, location string, group *DependencyGroup) {
	maven, ok := asStringMap(raw)
	if !ok {
		groupDiagnostic(group, location+".maven: must be a mapping")
		return
	}
	coordinate, ok := maven["coordinates"].(string)
	if !ok || strings.TrimSpace(coordinate) == "" {
		groupDiagnostic(group, location+".maven.coordinates: must be a non-empty literal string")
		return
	}
	parts := strings.Split(coordinate, ":")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" || strings.Contains(coordinate, "${") {
		groupDiagnostic(group, fmt.Sprintf("%s.maven.coordinates %q: expected literal groupId:artifactId:version", location, coordinate))
		return
	}
	if repo, exists := maven["repo"]; exists && repo != nil {
		groupDiagnostic(group, location+".maven.repo: custom repositories are unsupported for Maven generation")
		return
	}
	dep := DependencyReference{PackageType: "maven", Raw: coordinate, Name: parts[0] + ":" + parts[1], VersionConstraint: parts[2], SourceGroup: group.Name, OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime}
	if rawExclusions, exists := maven["exclusions"]; exists {
		exclusions, ok := rawExclusions.([]any)
		if !ok {
			groupDiagnostic(group, location+".maven.exclusions: must be a list")
			return
		}
		for i, rawExclusion := range exclusions {
			value, ok := rawExclusion.(string)
			exclusionParts := strings.Split(value, ":")
			if !ok || len(exclusionParts) != 2 || exclusionParts[0] == "" || exclusionParts[1] == "" || strings.Contains(value, "${") {
				groupDiagnostic(group, fmt.Sprintf("%s.maven.exclusions[%d]: expected literal groupId:artifactId", location, i))
				return
			}
			dep.MavenExclusions = append(dep.MavenExclusions, MavenExclusion{GroupID: exclusionParts[0], ArtifactID: exclusionParts[1]})
		}
	}
	group.Dependencies = append(group.Dependencies, dep)
}
