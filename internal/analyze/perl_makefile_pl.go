package analyze

import (
	"regexp"
)

type perlMakefilePLMatcherConfig struct{}

type perlMakefilePLParser struct{}

type perlMakefilePLPrerequisiteGroup struct {
	name  string
	scope DependencyScope
}

var (
	perlMakefilePLUsesModuleInstall = regexp.MustCompile(`(?m)^\s*use\s+(?:inc::)?Module::Install\b`)
	perlMakefilePLWriteAll          = regexp.MustCompile(`\bWriteAll\b`)
	perlMakefilePLUsesExtUtils      = regexp.MustCompile(`(?m)^\s*use\s+ExtUtils::MakeMaker\b`)
	perlMakefilePLWriteMakefile     = regexp.MustCompile(`\bWriteMakefile\s*\(`)
	perlMakefilePLModuleInstallCall = regexp.MustCompile(`(?ms)^\s*(requires|recommends|build_requires|configure_requires|test_requires)\b\s*(?:\(\s*)?((?:(?:'(?:(?:\\.)|[^'])*'|"(?:(?:\\.)|[^"])*"|[A-Za-z_][A-Za-z0-9_:]*)\s*(?:=>|,)\s*(?:'(?:(?:\\.)|[^'])*'|"(?:(?:\\.)|[^"])*"|[vV]?\d[A-Za-z0-9._+\-]*))(?:\s*,\s*(?:'(?:(?:\\.)|[^'])*'|"(?:(?:\\.)|[^"])*"|[A-Za-z_][A-Za-z0-9_:]*)\s*(?:=>|,)\s*(?:'(?:(?:\\.)|[^'])*'|"(?:(?:\\.)|[^"])*"|[vV]?\d[A-Za-z0-9._+\-]*))*\s*,?)\s*\)?\s*;`)
	perlMakefilePLExtUtilsGroup     = regexp.MustCompile(`(?i)\b(PREREQ_PM|BUILD_REQUIRES|CONFIGURE_REQUIRES|TEST_REQUIRES|RECOMMENDS)\s*=>\s*\{`)
	perlMakefilePLPrerequisite      = regexp.MustCompile(`(?:'((?:\\.|[^'])*)'|"((?:\\.|[^"])*)"|([A-Za-z_][A-Za-z0-9_:]*))\s*=>\s*(?:'((?:\\.|[^'])*)'|"((?:\\.|[^"])*)"|([vV]?\d[A-Za-z0-9._+\-]*))`)
)

var perlMakefilePLPrerequisiteGroups = map[string]perlMakefilePLPrerequisiteGroup{
	"requires":           {name: "requires", scope: ScopeRuntime},
	"recommends":         {name: "recommends", scope: ScopeOptional},
	"build_requires":     {name: "build_requires", scope: ScopeBuild},
	"configure_requires": {name: "configure_requires", scope: ScopeBuild},
	"test_requires":      {name: "test_requires", scope: ScopeTest},
	"PREREQ_PM":          {name: "PREREQ_PM", scope: ScopeRuntime},
	"BUILD_REQUIRES":     {name: "BUILD_REQUIRES", scope: ScopeBuild},
	"CONFIGURE_REQUIRES": {name: "CONFIGURE_REQUIRES", scope: ScopeBuild},
	"TEST_REQUIRES":      {name: "TEST_REQUIRES", scope: ScopeTest},
	"RECOMMENDS":         {name: "RECOMMENDS", scope: ScopeOptional},
}

func newPerlMakefilePLParser(perlMakefilePLMatcherConfig) (sourceAnalyzer, error) {
	return perlMakefilePLParser{}, nil
}

func (perlMakefilePLParser) Analyze(_ string, content []byte) (sourceAnalyzerResult, error) {
	cleaned := perlBuildPLWithoutComments(string(content))
	var dependencies []DependencyReference

	switch {
	case perlMakefilePLUsesModuleInstall.MatchString(cleaned) && perlMakefilePLWriteAll.MatchString(cleaned):
		dependencies = perlMakefilePLModuleInstallDependencies(cleaned)
	case perlMakefilePLUsesExtUtils.MatchString(cleaned) && perlMakefilePLWriteMakefile.MatchString(cleaned):
		dependencies = perlMakefilePLExtUtilsDependencies(cleaned)
	default:
		return sourceAnalyzerResult{}, nil
	}

	sortDependencyReferences(dependencies)
	return semanticAnalyzerResult(dependencies, nil), nil
}

func perlMakefilePLModuleInstallDependencies(content string) []DependencyReference {
	dependencies := make([]DependencyReference, 0)
	seen := make(map[string]struct{})
	for _, match := range perlMakefilePLModuleInstallCall.FindAllStringSubmatch(content, -1) {
		group, ok := perlMakefilePLPrerequisiteGroups[match[1]]
		if !ok {
			continue
		}
		dependencies = perlMakefilePLAppendPrerequisites(dependencies, seen, match[2], group)
	}
	return dependencies
}

func perlMakefilePLExtUtilsDependencies(content string) []DependencyReference {
	dependencies := make([]DependencyReference, 0)
	seen := make(map[string]struct{})
	for _, call := range perlMakefilePLWriteMakefile.FindAllStringIndex(content, -1) {
		arguments, ok := perlBuildPLDelimitedBody(content, call[1]-1, '(', ')')
		if !ok {
			continue
		}
		for _, match := range perlMakefilePLExtUtilsGroup.FindAllStringSubmatchIndex(arguments, -1) {
			group, ok := perlMakefilePLPrerequisiteGroups[perlBuildPLSubmatch(arguments, match, 1)]
			if !ok {
				continue
			}
			prerequisites, ok := perlBuildPLDelimitedBody(arguments, match[1]-1, '{', '}')
			if !ok {
				continue
			}
			dependencies = perlMakefilePLAppendPrerequisites(dependencies, seen, prerequisites, group)
		}
	}
	return dependencies
}

func perlMakefilePLAppendPrerequisites(dependencies []DependencyReference, seen map[string]struct{}, content string, group perlMakefilePLPrerequisiteGroup) []DependencyReference {
	for _, match := range perlMakefilePLPrerequisite.FindAllStringSubmatch(content, -1) {
		name := firstNonEmpty(match[1], match[2], match[3])
		constraint := firstNonEmpty(match[4], match[5], match[6])
		if name == "" {
			continue
		}
		dependency := perlMakefilePLDependency(name, constraint, group)
		key := dependency.SourceGroup + "\x00" + dependency.Name + "\x00" + dependency.VersionConstraint
		dependencies = appendUniqueDependency(dependencies, seen, key, dependency)
	}
	return dependencies
}

func perlMakefilePLDependency(name, constraint string, group perlMakefilePLPrerequisiteGroup) DependencyReference {
	dependency := DependencyReference{
		PackageType:  "cpan",
		Raw:          name,
		Name:         name,
		SourceGroup:  group.name,
		OriginKind:   OriginRegistry,
		Relationship: RelationshipDirect,
		Scope:        group.scope,
	}
	if constraint != "" {
		dependency.Raw += "@" + constraint
		dependency.VersionConstraint = constraint
	}
	return dependency
}
