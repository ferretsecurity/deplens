package analyze

import (
	"strings"

	"github.com/git-pkgs/vers"
)

// applyDependencyVERS derives canonical VERS URIs from extracted native
// constraints. Constraints that cannot be represented are intentionally left
// without VERS so callers can continue to use the original raw constraint.
func applyDependencyVERS(dependencies []DependencyReference) {
	for idx := range dependencies {
		dependencies[idx].VERS = dependencyVERS(dependencies[idx].PackageType, dependencies[idx].VersionConstraint)
	}
}

func dependencyVERS(packageType PackageType, versionConstraint string) string {
	if strings.Contains(versionConstraint, "${") || strings.Contains(versionConstraint, "$(") {
		return ""
	}
	nativeScheme, outputScheme, ok := versSchemeForPackageType(packageType)
	if !ok || versionConstraint == "" {
		return ""
	}

	parsed, err := vers.ParseNative(versionConstraint, nativeScheme)
	if err != nil {
		return ""
	}
	return vers.ToVersString(parsed, outputScheme)
}

func versSchemeForPackageType(packageType PackageType) (nativeScheme, outputScheme string, ok bool) {
	switch packageType {
	case "pypi", "golang", "npm", "maven", "gem", "cargo":
		return string(packageType), string(packageType), true
	default:
		return "", "", false
	}
}
