package analyze

import "github.com/git-pkgs/vers"

// applyDependencyVERS derives canonical VERS URIs from extracted native
// constraints. Constraints that cannot be represented are intentionally left
// without VERS so callers can continue to use the original raw constraint.
func applyDependencyVERS(dependencies []Dependency) {
	for idx := range dependencies {
		dependencies[idx].VERS = dependencyVERS(dependencies[idx].Type, dependencies[idx].Constraint)
	}
}

func dependencyVERS(packageType PackageType, constraint string) string {
	nativeScheme, outputScheme, ok := versSchemeForPackageType(packageType)
	if !ok || constraint == "" {
		return ""
	}

	parsed, err := vers.ParseNative(constraint, nativeScheme)
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
