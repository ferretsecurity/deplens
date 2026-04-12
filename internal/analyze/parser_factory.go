package analyze

import "fmt"

func compileManifestParser(raw ruleConfig) (manifestParser, error) {
	parserCount := 0
	if raw.BannerRegex != "" {
		parserCount++
	}
	if raw.Terraform != nil {
		parserCount++
	}
	if raw.INI != nil {
		parserCount++
	}
	if raw.TypeScript != nil {
		parserCount++
	}
	if raw.Python != nil {
		parserCount++
	}
	if raw.PyRequirements != nil {
		parserCount++
	}
	if raw.PoetryLock != nil {
		parserCount++
	}
	if raw.UVLock != nil {
		parserCount++
	}
	if raw.YAML != nil {
		parserCount++
	}
	if raw.TOML != nil {
		parserCount++
	}
	if raw.JSON != nil {
		parserCount++
	}
	if raw.XML != nil {
		parserCount++
	}
	if raw.HTML != nil {
		parserCount++
	}
	if parserCount > 1 {
		return nil, fmt.Errorf("exactly one parser type may be configured")
	}
	if raw.BannerRegex != "" {
		return newBannerRegexParser(raw.BannerRegex)
	}
	if raw.Terraform != nil {
		return newTerraformResourceParser(*raw.Terraform)
	}
	if raw.INI != nil {
		return newINIQueryParser(*raw.INI)
	}
	if raw.TypeScript != nil {
		return newTypeScriptMatcher(*raw.TypeScript)
	}
	if raw.Python != nil {
		return newPythonMatcher(*raw.Python)
	}
	if raw.PyRequirements != nil {
		return newPyRequirementsMatcher(*raw.PyRequirements)
	}
	if raw.PoetryLock != nil {
		return newPoetryLockParser(*raw.PoetryLock)
	}
	if raw.UVLock != nil {
		return newUVLockParser(*raw.UVLock)
	}
	if raw.YAML != nil {
		return newYAMLQueryParser(*raw.YAML)
	}
	if raw.TOML != nil {
		return newTOMLQueryParser(*raw.TOML)
	}
	if raw.JSON != nil {
		return newJSONMatcher(*raw.JSON)
	}
	if raw.XML != nil {
		return newXMLMatcher(*raw.XML)
	}
	if raw.HTML != nil {
		return newHTMLMatcher(*raw.HTML)
	}
	return nil, nil
}
