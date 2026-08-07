package analyze

import (
	"encoding/json"
	"fmt"
)

// DetectorInventoryEntry is the stable, semantic detector description used by
// repository development tooling. It intentionally excludes implementation
// pointers and compiled selector state.
type DetectorInventoryEntry struct {
	ID             string
	Form           string
	Roles          []string
	FilenameRegex  string
	PathGlob       string
	Analyzer       string
	AnalyzerConfig string
	Capabilities   []string
}

// DefaultDetectorInventory returns entries in the default ruleset's declared
// order. Callers should use Capabilities rather than guessing from analyzer
// presence: an analyzer may recognize or assess a source without extracting
// references.
func DefaultDetectorInventory() ([]DetectorInventoryEntry, error) {
	rules, err := LoadDefaultRules()
	if err != nil {
		return nil, err
	}
	entries := make([]DetectorInventoryEntry, 0, len(rules.detectors))
	for _, d := range rules.detectors {
		roles := make([]string, len(d.Roles))
		for i, role := range d.Roles {
			roles[i] = string(role)
		}
		entries = append(entries, DetectorInventoryEntry{
			ID: string(d.ID), Form: string(d.Form), Roles: roles,
			FilenameRegex: selectorRegex(d), PathGlob: d.PathGlob, Analyzer: d.AnalyzerType, AnalyzerConfig: d.AnalyzerConfig,
			Capabilities: append([]string(nil), d.Capabilities...),
		})
	}
	return entries, nil
}

func semanticAnalyzerConfig(raw *analyzerConfig) (string, error) {
	if raw == nil {
		return "", nil
	}
	var config map[string]any
	if err := raw.config.Decode(&config); err != nil {
		return "", fmt.Errorf("decode configuration: %w", err)
	}
	config["type"] = raw.Type
	encoded, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("encode configuration: %w", err)
	}
	return string(encoded), nil
}

func analyzerType(raw ruleConfig) string {
	if raw.Analyzer == nil {
		return ""
	}
	return raw.Analyzer.Type
}

func selectorRegex(d detector) string {
	if d.FilenameRegexp == nil {
		return ""
	}
	return d.FilenameRegexp.String()
}

// classifyDetectorCapabilities is the single source of truth for the
// collection-relevant capabilities of a configured analyzer.
func classifyDetectorCapabilities(raw *analyzerConfig) ([]string, error) {
	capabilities := []string{"select"}
	if raw == nil {
		return capabilities, nil
	}
	capabilities = append(capabilities, "recognize")
	extracts, err := analyzerExtractsReferences(raw)
	if err != nil {
		return nil, err
	}
	if extracts {
		return append(capabilities, "extract", "normalize"), nil
	}
	return append(capabilities, "assess-presence"), nil
}

func analyzerExtractsReferences(raw *analyzerConfig) (bool, error) {
	switch raw.Type {
	case "py-requirements", "pipfile-lock", "poetry-lock", "uv-lock", "go-mod", "package-lock", "package-json", "pnpm-lock", "composer-lock", "cargo-lock", "yarn-lock", "gradle-build", "gradle-lock", "gradle-version-catalog", "gemfile", "gemfile-lock", "dockerfile", "docker-compose", "maven-pom", "cargo-manifest", "composer-manifest", "dotnet-project", "dotnet-central-packages", "dotnet-packages-config":
		return true, nil
	case "toml":
		var config tomlMatcherConfig
		if err := raw.config.Decode(&config); err != nil {
			return false, fmt.Errorf("decode toml configuration: %w", err)
		}
		return len(config.Queries) > 0 || len(config.TableQueries) > 0, nil
	case "yaml":
		var config yamlMatcherConfig
		if err := raw.config.Decode(&config); err != nil {
			return false, fmt.Errorf("decode yaml configuration: %w", err)
		}
		return config.Query != "", nil
	case "ini":
		return true, nil
	default:
		return false, nil
	}
}
