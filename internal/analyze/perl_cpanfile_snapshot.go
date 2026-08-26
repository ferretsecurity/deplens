package analyze

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

type perlCpanfileSnapshotParser struct{}

var perlCpanfileSnapshotHeader = regexp.MustCompile(`(?m)^# carton snapshot format: version 1\.0\s*$`)

func newPerlCpanfileSnapshotParser(perlCpanfileSnapshotMatcherConfig) (sourceAnalyzer, error) {
	return perlCpanfileSnapshotParser{}, nil
}

func (perlCpanfileSnapshotParser) Analyze(_ string, content []byte) (sourceAnalyzerResult, error) {
	text := string(content)
	if !perlCpanfileSnapshotHeader.MatchString(text) {
		return sourceAnalyzerResult{}, nil
	}

	inDistributions := false
	pendingDistribution := ""
	pendingLine := 0
	dependencies := make([]DependencyReference, 0)
	incomplete := make([]string, 0)
	seen := make(map[string]struct{})
	for lineNumber, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "DISTRIBUTIONS" {
			inDistributions = true
			continue
		}
		if !inDistributions {
			continue
		}

		if distribution, ok := perlCpanfileSnapshotDistribution(line); ok {
			if pendingDistribution != "" {
				incomplete = append(incomplete, fmt.Sprintf("cpanfile.snapshot distribution %q on line %d has no pathname", pendingDistribution, pendingLine))
			}
			pendingDistribution = distribution
			pendingLine = lineNumber + 1
			continue
		}

		pathname, ok := perlCpanfileSnapshotPathname(line)
		if !ok || pendingDistribution == "" {
			continue
		}
		dependency, ok := perlCpanfileSnapshotDependency(pathname)
		if !ok {
			incomplete = append(incomplete, fmt.Sprintf("cpanfile.snapshot distribution %q on line %d has an invalid pathname", pendingDistribution, lineNumber+1))
		} else {
			dependencies = appendUniqueDependency(dependencies, seen, dependency.Raw, dependency)
		}
		pendingDistribution = ""
		pendingLine = 0
	}
	if !inDistributions {
		return sourceAnalyzerResult{}, nil
	}
	if pendingDistribution != "" {
		incomplete = append(incomplete, fmt.Sprintf("cpanfile.snapshot distribution %q on line %d has no pathname", pendingDistribution, pendingLine))
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, incomplete), nil
}

func perlCpanfileSnapshotDistribution(line string) (string, bool) {
	if !strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "   ") {
		return "", false
	}
	name := strings.TrimSpace(line)
	if name == "" || strings.Contains(name, ":") {
		return "", false
	}
	return name, true
}

func perlCpanfileSnapshotPathname(line string) (string, bool) {
	if !strings.HasPrefix(line, "    pathname:") {
		return "", false
	}
	pathname := strings.TrimSpace(strings.TrimPrefix(line, "    pathname:"))
	return pathname, pathname != ""
}

func perlCpanfileSnapshotDependency(pathname string) (DependencyReference, bool) {
	filename := path.Base(strings.TrimSpace(pathname))
	for _, extension := range []string{".tar.gz", ".tar.bz2", ".tgz", ".zip"} {
		if strings.HasSuffix(filename, extension) {
			filename = strings.TrimSuffix(filename, extension)
			break
		}
	}

	for separator := strings.LastIndex(filename, "-"); separator >= 0; separator = strings.LastIndex(filename[:separator], "-") {
		name, version := filename[:separator], filename[separator+1:]
		if name == "" || !perlCpanfileSnapshotVersion(version) {
			continue
		}
		return DependencyReference{
			PackageType:  "cpan",
			Raw:          name + "@" + version,
			Name:         name,
			Version:      version,
			SourceGroup:  "DISTRIBUTIONS",
			OriginKind:   OriginRegistry,
			Relationship: RelationshipInconclusive,
			Scope:        ScopeRuntime,
		}, true
	}
	return DependencyReference{}, false
}

func perlCpanfileSnapshotVersion(version string) bool {
	if version == "" {
		return false
	}
	firstDigit := version[0]
	if firstDigit == 'v' || firstDigit == 'V' {
		if len(version) == 1 {
			return false
		}
		firstDigit = version[1]
	}
	if firstDigit < '0' || firstDigit > '9' {
		return false
	}
	for _, character := range version {
		if (character >= '0' && character <= '9') || (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || strings.ContainsRune("._+-", character) {
			continue
		}
		return false
	}
	return true
}
