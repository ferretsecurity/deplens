package analyze

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type generationPlan struct {
	destination string
	directory   string
	relative    string
	content     []byte
	outcome     GenerationOutcome
}

func GeneratePythonRequirements(result *ScanResult, overwrite bool) error {
	return Generate(result, GenerationPythonRequirements, overwrite)
}

func Generate(result *ScanResult, format GenerationFormat, overwrite bool) error {
	if format != GenerationPythonRequirements && format != GenerationMavenPOM {
		return fmt.Errorf("unsupported generation format %q", format)
	}
	generation := &GenerationResult{Format: format, Paths: []string{}, Outcomes: []GenerationOutcome{}}
	plans := []generationPlan{}
	for _, source := range result.Sources {
		formatGroups := false
		for _, group := range source.Groups {
			formatGroups = formatGroups || group.Format == format
		}
		if source.Generate != format && !formatGroups {
			continue
		}
		if !formatGroups && (source.Analysis.Extraction == ExtractionFailed || source.Analysis.Extraction == ExtractionPartial || source.Analysis.Extraction == ExtractionUnsupported) {
			detail := fmt.Sprintf("extraction is %s", source.Analysis.Extraction)
			if len(source.Diagnostics) > 0 {
				detail = source.Diagnostics[0].Message
			}
			return fmt.Errorf("cannot generate from %s (%s): %s", source.Path, source.Detector, detail)
		}
		if len(source.Groups) == 0 {
			return fmt.Errorf("cannot generate from %s (%s): analyzer returned no independent groups", source.Path, source.Detector)
		}
		groups := slices.Clone(source.Groups)
		if formatGroups {
			groups = slices.DeleteFunc(groups, func(group DependencyGroup) bool { return group.Format != format })
		}
		slices.SortFunc(groups, func(a, b DependencyGroup) int { return strings.Compare(a.Location, b.Location) })
		components := generationGroupComponents(groups)
		usedRelative := make(map[string]string, len(groups))
		for index, group := range groups {
			if len(group.Diagnostics) > 0 {
				return fmt.Errorf("cannot generate from %s (%s) group at %s: %s", source.Path, source.Detector, group.Location, group.Diagnostics[0].Message)
			}
			outcome := GenerationOutcome{Source: source.Path, Group: group.Name, Location: group.Location, Status: GenerationOutcomeStatus(group.State)}
			if group.State != GroupReady {
				generation.Outcomes = append(generation.Outcomes, outcome)
				continue
			}
			component := components[index]
			rel := source.Path + "-" + component + ".generated-requirements.txt"
			directory := ""
			if format == GenerationMavenPOM {
				directory = source.Path + "-" + component + ".generated-maven"
				rel = directory + "/pom.xml"
			}
			if previous, exists := usedRelative[rel]; exists {
				return fmt.Errorf("generated destination collision for %s groups at %s and %s", source.Path, previous, group.Location)
			}
			usedRelative[rel] = group.Location
			destination := filepath.Join(result.Root, filepath.FromSlash(rel))
			cleanRoot := filepath.Clean(result.Root) + string(filepath.Separator)
			if !strings.HasPrefix(filepath.Clean(destination)+string(filepath.Separator), cleanRoot) {
				return fmt.Errorf("generated destination for %s group %q escapes scan root", source.Path, group.Name)
			}
			if directory != "" {
				if err := validateGeneratedMavenDirectory(result.Root, directory, overwrite); err != nil {
					return err
				}
			}
			if info, err := os.Lstat(destination); err == nil {
				if !overwrite {
					return fmt.Errorf("generated destination already exists: %s", rel)
				}
				if !info.Mode().IsRegular() {
					return fmt.Errorf("generated destination is not a regular file: %s", rel)
				}
			} else if !os.IsNotExist(err) {
				return fmt.Errorf("inspect generated destination %s: %w", rel, err)
			}
			content := pythonRequirementsContent(group.Dependencies)
			if format == GenerationMavenPOM {
				var err error
				content, err = mavenPOMContent(group.Dependencies)
				if err != nil {
					return fmt.Errorf("cannot generate from %s (%s) group at %s: %w", source.Path, source.Detector, group.Location, err)
				}
			}
			outcome.Path = rel
			plans = append(plans, generationPlan{destination: destination, directory: directory, relative: rel, content: content, outcome: outcome})
		}
	}
	slices.SortFunc(plans, func(a, b generationPlan) int { return strings.Compare(a.relative, b.relative) })
	for _, plan := range plans {
		if plan.directory != "" {
			if err := os.MkdirAll(filepath.Join(result.Root, filepath.FromSlash(plan.directory)), 0o755); err != nil {
				return fmt.Errorf("create generated directory %s: %w", plan.directory, err)
			}
		}
		if err := writeGeneratedFile(plan, overwrite); err != nil {
			return err
		}
		generation.Paths = append(generation.Paths, plan.relative)
		generation.Outcomes = append(generation.Outcomes, plan.outcome)
	}
	slices.SortFunc(generation.Outcomes, func(a, b GenerationOutcome) int {
		if a.Source != b.Source {
			return strings.Compare(a.Source, b.Source)
		}
		return strings.Compare(a.Location, b.Location)
	})
	result.Generation = generation
	return nil
}

func pythonRequirementsContent(dependencies []DependencyReference) []byte {
	var lines strings.Builder
	for _, dep := range dependencies {
		lines.WriteString(dep.Raw)
		lines.WriteByte('\n')
	}
	return []byte(lines.String())
}

type pomProject struct {
	XMLName      xml.Name        `xml:"project"`
	XMLNS        string          `xml:"xmlns,attr"`
	ModelVersion string          `xml:"modelVersion"`
	GroupID      string          `xml:"groupId"`
	ArtifactID   string          `xml:"artifactId"`
	Version      string          `xml:"version"`
	Dependencies []pomDependency `xml:"dependencies>dependency"`
}
type pomDependency struct {
	GroupID    string         `xml:"groupId"`
	ArtifactID string         `xml:"artifactId"`
	Version    string         `xml:"version"`
	Exclusions *pomExclusions `xml:"exclusions,omitempty"`
}
type pomExclusions struct {
	Values []MavenExclusion `xml:"exclusion"`
}

func mavenPOMContent(dependencies []DependencyReference) ([]byte, error) {
	p := pomProject{XMLNS: "http://maven.apache.org/POM/4.0.0", ModelVersion: "4.0.0", GroupID: "dev.deplens.generated", ArtifactID: "dependencies", Version: "1.0.0"}
	for _, dep := range dependencies {
		parts := strings.SplitN(dep.Name, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" || dep.VersionConstraint == "" {
			return nil, fmt.Errorf("dependency %q does not contain groupId:artifactId and version", dep.Raw)
		}
		item := pomDependency{GroupID: parts[0], ArtifactID: parts[1], Version: dep.VersionConstraint}
		if len(dep.MavenExclusions) > 0 {
			item.Exclusions = &pomExclusions{Values: dep.MavenExclusions}
		}
		p.Dependencies = append(p.Dependencies, item)
	}
	body, _ := xml.MarshalIndent(p, "", "  ")
	return append([]byte(xml.Header), append(body, '\n')...), nil
}

func validateGeneratedMavenDirectory(root, relative string, overwrite bool) error {
	path := filepath.Join(root, filepath.FromSlash(relative))
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect generated directory %s: %w", relative, err)
	}
	if !overwrite {
		return fmt.Errorf("generated destination already exists: %s", relative)
	}
	if !info.IsDir() {
		return fmt.Errorf("generated destination is not a directory: %s", relative)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("inspect generated directory %s: %w", relative, err)
	}
	for _, entry := range entries {
		if entry.Name() != "pom.xml" {
			return fmt.Errorf("generated directory contains unmanaged entry: %s/%s", relative, entry.Name())
		}
	}
	return nil
}

// generationGroupComponents keeps ordinary names readable while making every
// selected source node addressable. Name conversion happens before collision
// detection so names such as "daily job" and "daily/job" are disambiguated.
func generationGroupComponents(groups []DependencyGroup) []string {
	base := make([]string, len(groups))
	counts := make(map[string]int, len(groups))
	for i, group := range groups {
		if group.Name == "" {
			base[i] = "group"
		} else {
			base[i] = safeFilenameComponent(group.Name)
		}
		counts[base[i]]++
	}
	components := make([]string, len(groups))
	used := make(map[string]struct{}, len(groups))
	for i, group := range groups {
		component := base[i]
		if group.Name == "" || counts[component] > 1 {
			component += "-at-" + locationFilenameComponent(group.Location)
		}
		candidate := component
		for suffix := 2; ; suffix++ {
			if _, exists := used[candidate]; !exists {
				break
			}
			candidate = fmt.Sprintf("%s-%d", component, suffix)
		}
		used[candidate] = struct{}{}
		components[i] = candidate
	}
	return components
}

func safeFilenameComponent(value string) string {
	var b strings.Builder
	separator := false
	for _, r := range value {
		safe := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.'
		if safe {
			b.WriteRune(r)
			separator = false
		} else if !separator {
			b.WriteByte('-')
			separator = true
		}
	}
	component := b.String()
	if component == "" || component == "." || component == ".." {
		return "group"
	}
	return component
}

func locationFilenameComponent(location string) string {
	component := strings.Trim(safeFilenameComponent(strings.TrimPrefix(location, ".")), "-.")
	if component == "group" {
		return "root"
	}
	return component
}

func writeGeneratedFile(plan generationPlan, overwrite bool) error {
	if !overwrite {
		file, err := os.OpenFile(plan.destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			return fmt.Errorf("create generated file %s: %w", plan.relative, err)
		}
		if _, err = file.Write(plan.content); err != nil {
			file.Close()
			return fmt.Errorf("write generated file %s: %w", plan.relative, err)
		}
		if err := file.Close(); err != nil {
			return fmt.Errorf("close generated file %s: %w", plan.relative, err)
		}
		return nil
	}

	temporary, err := os.CreateTemp(filepath.Dir(plan.destination), ".deplens-generated-*")
	if err != nil {
		return fmt.Errorf("create temporary generated file for %s: %w", plan.relative, err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o644); err != nil {
		temporary.Close()
		return fmt.Errorf("set permissions on temporary generated file for %s: %w", plan.relative, err)
	}
	if _, err := temporary.Write(plan.content); err != nil {
		temporary.Close()
		return fmt.Errorf("write temporary generated file for %s: %w", plan.relative, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary generated file for %s: %w", plan.relative, err)
	}
	if err := os.Rename(temporaryName, plan.destination); err != nil {
		return fmt.Errorf("replace generated file %s: %w", plan.relative, err)
	}
	return nil
}
