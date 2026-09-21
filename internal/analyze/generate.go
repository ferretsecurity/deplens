package analyze

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type generationPlan struct {
	destination string
	relative    string
	content     []byte
	outcome     GenerationOutcome
}

func GeneratePythonRequirements(result *ScanResult, overwrite bool) error {
	generation := &GenerationResult{Format: GenerationPythonRequirements, Paths: []string{}, Outcomes: []GenerationOutcome{}}
	plans := []generationPlan{}
	for _, source := range result.Sources {
		if source.Generate != GenerationPythonRequirements {
			continue
		}
		if source.Analysis.Extraction == ExtractionFailed || source.Analysis.Extraction == ExtractionPartial || source.Analysis.Extraction == ExtractionUnsupported {
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
		slices.SortFunc(groups, func(a, b DependencyGroup) int { return strings.Compare(a.Location, b.Location) })
		components := generationGroupComponents(groups)
		usedRelative := make(map[string]string, len(groups))
		for index, group := range groups {
			outcome := GenerationOutcome{Source: source.Path, Group: group.Name, Location: group.Location, Status: GenerationOutcomeStatus(group.State)}
			if group.State != GroupReady {
				generation.Outcomes = append(generation.Outcomes, outcome)
				continue
			}
			component := components[index]
			rel := source.Path + "-" + component + ".generated-requirements.txt"
			if previous, exists := usedRelative[rel]; exists {
				return fmt.Errorf("generated destination collision for %s groups at %s and %s", source.Path, previous, group.Location)
			}
			usedRelative[rel] = group.Location
			destination := filepath.Join(result.Root, filepath.FromSlash(rel))
			cleanRoot := filepath.Clean(result.Root) + string(filepath.Separator)
			if !strings.HasPrefix(filepath.Clean(destination)+string(filepath.Separator), cleanRoot) {
				return fmt.Errorf("generated destination for %s group %q escapes scan root", source.Path, group.Name)
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
			var lines strings.Builder
			for _, dep := range group.Dependencies {
				lines.WriteString(dep.Raw)
				lines.WriteByte('\n')
			}
			outcome.Path = rel
			plans = append(plans, generationPlan{destination: destination, relative: rel, content: []byte(lines.String()), outcome: outcome})
		}
	}
	slices.SortFunc(plans, func(a, b generationPlan) int { return strings.Compare(a.relative, b.relative) })
	for _, plan := range plans {
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
