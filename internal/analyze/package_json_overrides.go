package analyze

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
)

var npmOverrideName = regexp.MustCompile(`^(@[A-Za-z0-9.!~*'()_-]+/[A-Za-z0-9!~*'()_-]|[A-Za-z0-9!~*'()-])[A-Za-z0-9.!~*'()_-]*$`)

func packageJSONOverrides(raw json.RawMessage) ([]DependencyReference, []string) {
	var entries map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&entries); err != nil || entries == nil {
		return nil, []string{"overrides: expected an object of override selectors"}
	}

	var refs []DependencyReference
	var messages []string
	var walk func(map[string]any, []string, string)
	walk = func(entries map[string]any, parents []string, parentName string) {
		for _, key := range slices.Sorted(maps.Keys(entries)) {
			path, name := parents, parentName
			if key != "." {
				path = append(parents, key)
				// A leading @ belongs to the scope. The next @ starts the condition.
				name = key
				if len(key) > 1 {
					if separator := strings.IndexByte(key[1:], '@'); separator >= 0 {
						name = key[:separator+1]
					}
				}
				condition := strings.TrimPrefix(key[len(name):], "@")
				if !npmOverrideName.MatchString(name) || (condition != "" && !npmOverrideRegistrySpec(condition)) {
					messages = append(messages, fmt.Sprintf("overrides %q: unsupported package selector", path))
					continue
				}
				if children, ok := entries[key].(map[string]any); ok {
					walk(children, path, name)
					continue
				}
			}
			spec, ok := entries[key].(string)
			if !ok || name == "" {
				messages = append(messages, fmt.Sprintf("overrides %q (%s): expected a package and replacement specifier", path, key))
				continue
			}
			dep, supported := npmOverrideReplacement(name, spec)
			if !supported {
				messages = append(messages, fmt.Sprintf("overrides %q: unsupported replacement specifier %q", path, spec))
				continue
			}
			if len(path) > 1 || path[0] != name {
				if dep.Attributes == nil {
					dep.Attributes = make(map[string]string)
				}
				encoded, _ := json.Marshal(path)
				dep.Attributes["override_path"] = string(encoded)
			}
			refs = append(refs, dep)
		}
	}
	walk(entries, nil, "")
	return refs, messages
}

func npmOverrideReplacement(name, spec string) (DependencyReference, bool) {
	dep := packageJSONDependency(name, spec, "overrides", "")
	dep.Relationship = RelationshipInconclusive
	switch {
	case strings.HasPrefix(spec, "$"):
		ref := spec[1:]
		dep.Attributes = map[string]string{"spec_reference": ref}
		dep.OriginKind = ""
		dep.VersionConstraint = ""
		return dep, npmOverrideName.MatchString(ref)
	case spec == "":
		dep.OriginKind = OriginRegistry
		dep.VersionConstraint = "*"
	case strings.HasPrefix(spec, "npm:"):
		return dep, spec != "npm:" && npmOverrideName.MatchString(dep.Name) &&
			(dep.VersionConstraint == "" || npmOverrideRegistrySpec(dep.VersionConstraint))
	case dep.OriginKind == OriginRegistry:
		return dep, npmOverrideRegistrySpec(spec)
	case dep.OriginKind == OriginWorkspace || strings.HasPrefix(spec, "link:"):
		return dep, false
	}
	return dep, dep.OriginKind != ""
}

// The shared VERS converter accepts opaque strings. Only treat a multi-token
// spec as a range when each token looks like a comparator or a range separator.
var npmOverrideRangeToken = regexp.MustCompile(`^(-|[<>]=?|[=~^]|([<>]=?|[=~^])?v?([0-9]+|[xX*])(\.([0-9]+|[xX*])){0,2}([-+][A-Za-z0-9.+-]+)?)$`)

func npmOverrideRegistrySpec(spec string) bool {
	if packageJSONRegistryTag.MatchString(spec) {
		return true
	}
	for _, token := range strings.Fields(strings.ReplaceAll(spec, "||", " ")) {
		if !npmOverrideRangeToken.MatchString(token) {
			return false
		}
	}
	return isPackageJSONRegistrySpecifier(spec)
}
