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
	var incomplete []string
	pythonDeclared := false
	recognized := false

	inspectScope := func(scope any, segments []any) {
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
			incomplete = append(incomplete, jqPath(append(append([]any{}, segments...), "resources"))+": must be a mapping")
			return
		}
		jobs, exists := resourcesMap["jobs"]
		if !exists {
			return
		}
		jobsMap, ok := asStringMap(jobs)
		if !ok {
			incomplete = append(incomplete, jqPath(append(append([]any{}, segments...), "resources", "jobs"))+": must be a mapping")
			return
		}
		for _, jobName := range sortedStringKeys(jobsMap) {
			rawJob := jobsMap[jobName]
			job, ok := asStringMap(rawJob)
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
				location := jqPath(append(append([]any{}, segments...), "resources", "jobs", jobName, "tasks"))
				incomplete = append(incomplete, location+": must be a list")
				continue
			}
			for taskIndex, rawTask := range tasks {
				locationSegments := append(append([]any{}, segments...), "resources", "jobs", jobName, "tasks", taskIndex)
				location := jqPath(locationSegments)
				task, ok := asStringMap(rawTask)
				if !ok {
					incomplete = append(incomplete, location+": task must be a mapping")
					continue
				}
				taskName, ok := task["task_key"].(string)
				if !ok || taskName == "" {
					incomplete = append(incomplete, location+".task_key: must be a non-empty string")
				}
				group := DependencyGroup{Name: taskName, Location: location}
				rawLibraries, exists := task["libraries"]
				if !exists || rawLibraries == nil {
					group.State = GroupMissing
					groups = append(groups, group)
					continue
				}
				libraries, ok := rawLibraries.([]any)
				if !ok {
					incomplete = append(incomplete, location+".libraries: must be a list")
					continue
				}
				group.State = GroupEmpty
				for libraryIndex, rawLibrary := range libraries {
					libraryLocation := fmt.Sprintf("%s.libraries[%d]", location, libraryIndex)
					library, ok := asStringMap(rawLibrary)
					if !ok {
						incomplete = append(incomplete, libraryLocation+": library must be a mapping")
						continue
					}
					if rawPyPI, exists := library["pypi"]; exists {
						pythonDeclared = true
						pypi, ok := asStringMap(rawPyPI)
						if !ok {
							incomplete = append(incomplete, libraryLocation+".pypi: must be a mapping")
							continue
						}
						packageSpec, ok := pypi["package"].(string)
						if !ok || strings.TrimSpace(packageSpec) == "" {
							incomplete = append(incomplete, libraryLocation+".pypi.package: must be a non-empty string")
							continue
						}
						if repo, exists := pypi["repo"]; exists && repo != nil {
							incomplete = append(incomplete, libraryLocation+".pypi.repo: custom repositories are unsupported for generation")
							continue
						}
						dependency, err := parsePythonRequirement(packageSpec)
						if err != nil {
							incomplete = append(incomplete, fmt.Sprintf("%s.pypi.package %q: %v", libraryLocation, packageSpec, err))
							continue
						}
						dependency.SourceGroup = taskName
						group.Dependencies = append(group.Dependencies, dependency)
						dependencies = append(dependencies, dependency)
						group.State = GroupReady
						continue
					}
					if _, exists := library["whl"]; exists {
						pythonDeclared = true
						incomplete = append(incomplete, libraryLocation+".whl: wheel declarations are unsupported for generation")
					}
					// Maven and other Databricks library kinds belong to other
					// ecosystems. They do not make Python extraction incomplete.
				}
				groups = append(groups, group)
			}
		}
	}

	inspectScope(rootMap, nil)
	if rawTargets, exists := rootMap["targets"]; exists {
		if targets, ok := asStringMap(rawTargets); ok {
			for _, targetName := range sortedStringKeys(targets) {
				target := targets[targetName]
				inspectScope(target, []any{"targets", targetName})
			}
		} else {
			incomplete = append(incomplete, ".targets: must be a mapping")
		}
	}
	if !recognized {
		return sourceAnalyzerResult{}, nil
	}

	presence := PresenceAbsent
	if pythonDeclared || len(dependencies) > 0 {
		presence = PresencePresent
	}
	if len(incomplete) > 0 {
		extraction := ExtractionFailed
		severity := DiagnosticError
		if len(dependencies) > 0 {
			extraction = ExtractionPartial
			severity = DiagnosticWarning
		}
		return sourceAnalyzerResult{
			Recognized:   true,
			Analysis:     SourceAnalysis{Presence: presence, Extraction: extraction},
			Dependencies: dependencies,
			Groups:       groups,
			Diagnostics:  []Diagnostic{{Severity: severity, Code: "databricks-bundle-incomplete", Message: strings.Join(incomplete, "; ")}},
		}, nil
	}
	return sourceAnalyzerResult{Recognized: true, Analysis: SourceAnalysis{Presence: presence, Extraction: ExtractionComplete}, Dependencies: dependencies, Groups: groups}, nil
}
