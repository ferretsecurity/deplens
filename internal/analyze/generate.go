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

func GeneratePythonRequirements(result *ScanResult) error {
	generation := &GenerationResult{Format: "python-requirements", Paths: []string{}, Outcomes: []GenerationOutcome{}}
	plans := []generationPlan{}
	for _, source := range result.Sources {
		if source.Generate != "python-requirements" {
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
		for _, group := range source.Groups {
			outcome := GenerationOutcome{Source: source.Path, Group: group.Name, Location: group.Location, Status: group.State}
			if group.State != "ready" {
				generation.Outcomes = append(generation.Outcomes, outcome)
				continue
			}
			rel := source.Path + "-" + group.Name + ".generated-requirements.txt"
			destination := filepath.Join(result.Root, filepath.FromSlash(rel))
			cleanRoot := filepath.Clean(result.Root) + string(filepath.Separator)
			if !strings.HasPrefix(filepath.Clean(destination)+string(filepath.Separator), cleanRoot) {
				return fmt.Errorf("generated destination for %s group %q escapes scan root", source.Path, group.Name)
			}
			if _, err := os.Lstat(destination); err == nil {
				return fmt.Errorf("generated destination already exists: %s", rel)
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
