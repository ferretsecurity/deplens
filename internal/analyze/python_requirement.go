package analyze

import (
	"fmt"
	"regexp"
	"strings"
)

var pythonRequirementPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*(?:\[[A-Za-z0-9._-]+(?:\s*,\s*[A-Za-z0-9._-]+)*\])?(?:\s*(?:===|==|!=|~=|<=|>=|<|>)\s*[^,;\s]+(?:\s*,\s*(?:===|==|!=|~=|<=|>=|<|>)\s*[^,;\s]+)*)?(?:\s*;\s*[^\r\n]+)?$`)

func parsePythonRequirement(raw string) (DependencyReference, error) {
	spec := strings.TrimSpace(raw)
	if spec == "" || strings.HasPrefix(spec, "-") || strings.HasPrefix(spec, "@") || strings.Contains(spec, " @ ") || strings.Contains(spec, "://") || strings.Contains(spec, "${") || strings.Contains(spec, "{{") || strings.HasPrefix(spec, ".") || strings.HasPrefix(spec, "/") || !pythonRequirementPattern.MatchString(spec) {
		return DependencyReference{}, fmt.Errorf("unsupported or malformed Python requirement")
	}
	parsed := parsePEP508Dep(spec)
	if parsed.name == "" {
		return DependencyReference{}, fmt.Errorf("unsupported or malformed Python requirement")
	}
	return DependencyReference{PackageType: "pypi", Raw: spec, Name: parsed.name, VersionConstraint: parsed.versionConstraint, OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: parsed.attributes}, nil
}
