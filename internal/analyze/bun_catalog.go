package analyze

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"strings"
)

func readBunCatalogs(content []byte) (map[string]map[string]string, error) {
	var fields map[string]jsontext.Value
	if err := json.Unmarshal(content, &fields); err != nil {
		return nil, err
	}
	workspace := fields["workspaces"]
	packages := workspace
	switch workspace.Kind() {
	case jsontext.KindBeginObject:
		var object map[string]jsontext.Value
		if err := json.Unmarshal(workspace, &object); err != nil {
			return nil, err
		}
		packages = object["packages"]
		if object["catalog"] != nil || object["catalogs"] != nil {
			fields = object
		}
	case jsontext.KindBeginArray:
	default:
		return nil, fmt.Errorf("workspaces must be an array or object")
	}
	if packages != nil {
		var patterns []jsontext.Value
		if packages.Kind() != jsontext.KindBeginArray || json.Unmarshal(packages, &patterns) != nil {
			return nil, fmt.Errorf("workspace packages must be an array of strings")
		}
		for _, pattern := range patterns {
			if pattern.Kind() != jsontext.KindString {
				return nil, fmt.Errorf("workspace packages must be strings")
			}
		}
	}

	catalogs := make(map[string]map[string]string)
	if raw := fields["catalogs"]; raw != nil {
		if err := json.Unmarshal(raw, &catalogs); err != nil {
			return nil, fmt.Errorf("catalogs: %w", err)
		}
		if catalogs == nil {
			return nil, fmt.Errorf("catalogs must be an object")
		}
	}
	for _, name := range sortedStringKeys(catalogs) {
		if catalogs[name] == nil {
			return nil, fmt.Errorf("catalogs.%s must be an object", name)
		}
	}
	defaults := catalogs["default"]
	if defaults == nil {
		defaults = make(map[string]string)
	}
	var singular map[string]string
	if raw := fields["catalog"]; raw != nil {
		if err := json.Unmarshal(raw, &singular); err != nil {
			return nil, fmt.Errorf("catalog: %w", err)
		}
		if singular == nil {
			return nil, fmt.Errorf("catalog must be an object")
		}
	}
	for _, entries := range []map[string]string{singular, catalogs[""]} {
		for _, key := range sortedStringKeys(entries) {
			if _, duplicate := defaults[key]; duplicate {
				return nil, fmt.Errorf("duplicate default catalog entry %q", key)
			}
			defaults[key] = entries[key]
		}
	}
	delete(catalogs, "")
	catalogs["default"] = defaults
	for _, name := range sortedStringKeys(catalogs) {
		for _, key := range sortedStringKeys(catalogs[name]) {
			if key == "" || strings.TrimSpace(catalogs[name][key]) == "" {
				return nil, fmt.Errorf("catalog %q entry %q: expected a non-empty dependency name and specifier", name, key)
			}
		}
	}
	return catalogs, nil
}

func validBunCatalogTarget(target string, dependency DependencyReference) bool {
	if strings.HasPrefix(target, "npm:") {
		return npmOverrideName.MatchString(dependency.Name) &&
			(dependency.VersionConstraint == "" || npmOverrideRegistrySpec(dependency.VersionConstraint)) &&
			target != "npm:"
	}
	if dependency.OriginKind == OriginRegistry {
		return npmOverrideRegistrySpec(target)
	}
	return true
}
