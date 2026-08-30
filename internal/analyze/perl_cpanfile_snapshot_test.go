package analyze

import (
	"path/filepath"
	"testing"
)

func TestPerlCpanfileSnapshotFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []DependencyReference
	}{
		{
			name:    "hyphenated distribution",
			fixture: "cpanfile-snapshot-dbd-mysql",
			want:    []DependencyReference{perlCpanfileSnapshotTestDependency("DBD-mysql", "4.043")},
		},
		{
			name:    "large version",
			fixture: "cpanfile-snapshot-file-slurp",
			want:    []DependencyReference{perlCpanfileSnapshotTestDependency("File-Slurp", "9999.32")},
		},
		{
			name:    "tgz archive",
			fixture: "cpanfile-snapshot-config-tiny",
			want:    []DependencyReference{perlCpanfileSnapshotTestDependency("Config-Tiny", "2.26")},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "perl", tc.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "cpanfile.snapshot")
			if source.Detector != "perl-cpanfile-snapshot" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, tc.want)
			}
		})
	}
}

func TestPerlCpanfileSnapshotWithoutDistributionsIsAbsent(t *testing.T) {
	parser, err := newPerlCpanfileSnapshotParser(perlCpanfileSnapshotMatcherConfig{})
	if err != nil {
		t.Fatalf("newPerlCpanfileSnapshotParser: %v", err)
	}
	result, err := parser.Analyze("cpanfile.snapshot", []byte("# carton snapshot format: version 1.0\nDISTRIBUTIONS\n"))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !result.Recognized || result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(result.Dependencies) != 0 {
		t.Fatalf("result = %+v", result)
	}
}

func perlCpanfileSnapshotTestDependency(name, version string) DependencyReference {
	return DependencyReference{
		PackageType:  "cpan",
		Raw:          name + "@" + version,
		Name:         name,
		Version:      version,
		SourceGroup:  "DISTRIBUTIONS",
		OriginKind:   OriginRegistry,
		Relationship: RelationshipInconclusive,
		Scope:        ScopeRuntime,
	}
}
