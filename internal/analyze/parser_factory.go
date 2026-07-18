package analyze

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

type bannerRegexAnalyzerConfig struct {
	Pattern string `yaml:"pattern"`
}

func compileSourceAnalyzer(raw ruleConfig) (sourceAnalyzer, error) {
	if raw.Analyzer == nil {
		return nil, nil
	}
	if raw.Analyzer.Type == "" {
		return nil, fmt.Errorf("type: required")
	}

	switch raw.Analyzer.Type {
	case "banner-regex":
		var config bannerRegexAnalyzerConfig
		if err := raw.Analyzer.decodeStrict(&config); err != nil {
			return nil, err
		}
		if config.Pattern == "" {
			return nil, fmt.Errorf("pattern: required")
		}
		return newBannerRegexParser(config.Pattern)
	case "terraform":
		return decodeAndCreate(raw.Analyzer, newTerraformResourceParser)
	case "ini":
		return decodeAndCreate(raw.Analyzer, newINIQueryParser)
	case "typescript":
		return decodeAndCreate(raw.Analyzer, newTypeScriptMatcher)
	case "python":
		return decodeAndCreate(raw.Analyzer, newPythonMatcher)
	case "py-requirements":
		return decodeAndCreate(raw.Analyzer, newPyRequirementsMatcher)
	case "pipfile-lock":
		return decodeAndCreate(raw.Analyzer, newPipfileLockParser)
	case "poetry-lock":
		return decodeAndCreate(raw.Analyzer, newPoetryLockParser)
	case "uv-lock":
		return decodeAndCreate(raw.Analyzer, newUVLockParser)
	case "go-mod":
		return decodeAndCreate(raw.Analyzer, newGoModMatcher)
	case "package-lock":
		return decodeAndCreate(raw.Analyzer, newPackageLockParser)
	case "pnpm-lock":
		return decodeAndCreate(raw.Analyzer, newPNPMLockParser)
	case "composer-lock":
		return decodeAndCreate(raw.Analyzer, newComposerLockParser)
	case "cargo-lock":
		return decodeAndCreate(raw.Analyzer, newCargoLockParser)
	case "yarn-lock":
		return decodeAndCreate(raw.Analyzer, newYarnLockParser)
	case "yaml":
		return decodeAndCreate(raw.Analyzer, newYAMLQueryParser)
	case "toml":
		return decodeAndCreate(raw.Analyzer, newTOMLQueryParser)
	case "json":
		return decodeAndCreate(raw.Analyzer, newJSONMatcher)
	case "xml":
		return decodeAndCreate(raw.Analyzer, newXMLMatcher)
	case "html":
		return decodeAndCreate(raw.Analyzer, newHTMLMatcher)
	default:
		return nil, fmt.Errorf("type: unsupported value %q", raw.Analyzer.Type)
	}
}

func (c analyzerConfig) decodeStrict(target any) error {
	data, err := yaml.Marshal(&c.config)
	if err != nil {
		return fmt.Errorf("encode configuration: %w", err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("%s configuration: %w", c.Type, err)
	}
	return nil
}

func decodeAndCreate[T any](config *analyzerConfig, create func(T) (sourceAnalyzer, error)) (sourceAnalyzer, error) {
	var value T
	if err := config.decodeStrict(&value); err != nil {
		return nil, err
	}
	return create(value)
}
