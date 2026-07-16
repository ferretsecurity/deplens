package analyze

import (
	"testing"

	"github.com/git-pkgs/vers"
)

func TestDependencyVERSConvertsThirtyNativeConstraints(t *testing.T) {
	testCases := []struct {
		name        string
		packageType PackageType
		constraint  string
		want        string
	}{
		// PyPI
		{"pypi exact", "pypi", "==1.2.3", "vers:pypi/%3D1.2.3"},
		{"pypi compatible", "pypi", "~=1.4.2", "vers:pypi/>=1.4.2|<1.5"},
		{"pypi bounded", "pypi", ">=1.0,<2.0", "vers:pypi/>=1.0|<2.0"},
		{"pypi exclusion", "pypi", "!=1.5.0", "vers:pypi/!=1.5.0"},
		{"pypi wildcard", "pypi", "==1.2.*", "vers:pypi/%3D1.2.%2A"},
		{"pypi prerelease", "pypi", "==1.0rc1", "vers:pypi/%3D1.0rc1"},
		// Go
		{"go exact", "golang", "v1.2.3", "vers:golang/v1.2.3"},
		{"go zero major", "golang", "v0.5.0", "vers:golang/v0.5.0"},
		{"go prerelease", "golang", "v1.2.3-rc.1", "vers:golang/v1.2.3-rc.1"},
		{"go major version", "golang", "v2.0.0", "vers:golang/v2.0.0"},
		// npm
		{"npm caret", "npm", "^1.2.3", "vers:npm/>=1.2.3|<2.0.0"},
		{"npm tilde", "npm", "~1.2.3", "vers:npm/>=1.2.3|<1.3.0"},
		{"npm x range", "npm", "1.2.x", "vers:npm/>=1.2.0|<1.3.0"},
		{"npm hyphen", "npm", "1.2.3 - 2.0.0", "vers:npm/>=1.2.3|<=2.0.0"},
		{"npm union", "npm", "^1.2.3 || ^2.0.0", "vers:npm/>=1.2.3|<2.0.0|>=2.0.0|<3.0.0"},
		{"npm comparators", "npm", ">=1.0.0 <2.0.0", "vers:npm/>=1.0.0|<2.0.0"},
		// Maven
		{"maven inclusive exclusive", "maven", "[1.0,2.0)", "vers:maven/>=1.0|<2.0"},
		{"maven exclusive inclusive", "maven", "(1.0,2.0]", "vers:maven/>1.0|<=2.0"},
		{"maven lower unbounded", "maven", "[1.0,)", "vers:maven/>=1.0"},
		{"maven exact", "maven", "[1.0]", "vers:maven/1.0"},
		{"maven upper unbounded", "maven", "(,2.0]", "vers:maven/<=2.0"},
		// RubyGems
		{"gem pessimistic", "gem", "~> 1.2.3", "vers:gem/>=1.2.3|<1.3"},
		{"gem bounded", "gem", ">= 1.0, < 2.0", "vers:gem/>=1.0|<2.0"},
		{"gem exclusion", "gem", "!= 1.5.0", "vers:gem/!=1.5.0"},
		{"gem exact", "gem", "= 1.2.3", "vers:gem/1.2.3"},
		// Cargo
		{"cargo implicit caret", "cargo", "1.2.3", "vers:cargo/>=1.2.3|<2.0.0"},
		{"cargo explicit caret", "cargo", "^1.2.3", "vers:cargo/>=1.2.3|<2.0.0"},
		{"cargo tilde", "cargo", "~1.2.3", "vers:cargo/>=1.2.3|<1.3.0"},
		{"cargo wildcard", "cargo", "1.2.*", "vers:cargo/>=1.2.0|<1.3.0"},
		{"cargo bounded", "cargo", ">=1.0,<2.0", "vers:cargo/>=1.0.0|<2.0.0"},
	}

	if len(testCases) != 30 {
		t.Fatalf("constraint corpus has %d cases, want 30", len(testCases))
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := dependencyVERS(tc.packageType, tc.constraint)
			if got != tc.want {
				t.Fatalf("dependencyVERS(%q, %q) = %q, want %q", tc.packageType, tc.constraint, got, tc.want)
			}
			if _, err := vers.Parse(got); err != nil {
				t.Fatalf("generated VERS %q does not parse: %v", got, err)
			}
		})
	}
}

func TestDependencyVERSRoundTripsRepresentativeRanges(t *testing.T) {
	testCases := []struct {
		packageType PackageType
		constraint  string
		included    string
		excluded    string
	}{
		{"pypi", ">=1.0,<2.0", "1.5", "2.0"},
		{"golang", "v1.2.3", "v1.2.3", "v2.0.0"},
		{"npm", "^1.2.3", "1.5.0", "2.0.0"},
		{"maven", "[1.0,2.0)", "1.5", "2.0"},
		{"gem", "~> 1.2.3", "1.2.9", "1.3.0"},
		{"cargo", "1.2.3", "1.5.0", "2.0.0"},
	}

	for _, tc := range testCases {
		t.Run(string(tc.packageType), func(t *testing.T) {
			got := dependencyVERS(tc.packageType, tc.constraint)
			parsed, err := vers.Parse(got)
			if err != nil {
				t.Fatalf("Parse(%q): %v", got, err)
			}
			if !parsed.Contains(tc.included) {
				t.Errorf("%q should contain %q", got, tc.included)
			}
			if parsed.Contains(tc.excluded) {
				t.Errorf("%q should not contain %q", got, tc.excluded)
			}
		})
	}
}

func TestDependencyVERSOmitsUnsupportedAndInvalidConstraints(t *testing.T) {
	for _, tc := range []struct {
		packageType PackageType
		constraint  string
	}{
		{packageType: "npm", constraint: "^"},
		{packageType: "composer", constraint: "^1.2.3"},
		{packageType: "docker", constraint: "^1.2.3"},
		{packageType: "npm", constraint: ""},
	} {
		if got := dependencyVERS(tc.packageType, tc.constraint); got != "" {
			t.Errorf("dependencyVERS(%q, %q) = %q, want empty", tc.packageType, tc.constraint, got)
		}
	}
}

func TestApplyDependencyVERS(t *testing.T) {
	dependencies := []Dependency{
		{Type: "pypi", Constraint: ">=2.31"},
		{Type: "npm", Version: "18.3.1"},
	}

	applyDependencyVERS(dependencies)

	if dependencies[0].VERS != "vers:pypi/>=2.31" {
		t.Fatalf("constraint VERS = %q", dependencies[0].VERS)
	}
	if dependencies[1].VERS != "" {
		t.Fatalf("resolved version produced VERS %q", dependencies[1].VERS)
	}
}
