package analyze

import (
	"slices"
	"strings"
	"testing"
)

func TestPythonCallLiteralCompleteness(t *testing.T) {
	for _, test := range []struct {
		name       string
		args       string
		want       []string
		presence   DependencyPresence
		extraction ExtractionState
	}{
		{"literal", `install_requires=["requests>=2"], extras_require={"test": ["pytest>=8"]}`, []string{"requests>=2", "pytest>=8"}, PresencePresent, ExtractionComplete},
		{"empty list", `install_requires=[]`, nil, PresenceAbsent, ExtractionComplete},
		{"empty dict", `extras_require={}`, nil, PresenceAbsent, ExtractionComplete},
		{"empty extra", `extras_require={"test": []}`, nil, PresenceAbsent, ExtractionComplete},
		{"dynamic only", `install_requires=requirements`, nil, PresenceUnknown, ExtractionFailed},
		{"dynamic with extras", `install_requires=requirements, extras_require={"test": ["pytest>=8"]}`, []string{"pytest>=8"}, PresencePresent, ExtractionPartial},
		{"dynamic extras", `install_requires=["requests>=2"], extras_require=extras`, []string{"requests>=2"}, PresencePresent, ExtractionPartial},
		{"partial list", `install_requires=["requests>=2", dynamic]`, []string{"requests>=2"}, PresencePresent, ExtractionPartial},
		{"partial dict", `extras_require={"test": ["pytest>=8"], "docs": dynamic}`, []string{"pytest>=8"}, PresencePresent, ExtractionPartial},
		{"formatted literal", `install_requires=[f"requests"]`, nil, PresenceUnknown, ExtractionFailed},
		{"bytes literal", `install_requires=[b"requests"]`, nil, PresenceUnknown, ExtractionFailed},
		{"concatenation", `install_requires=["requests" + ">=2"]`, nil, PresenceUnknown, ExtractionFailed},
		{"adjacent strings", `install_requires=["requests" ">=2"]`, nil, PresenceUnknown, ExtractionFailed},
	} {
		t.Run(test.name, func(t *testing.T) {
			rules := mustLoadDefaultRules(t)
			var parser sourceAnalyzer
			for _, detector := range rules.detectors {
				if detector.ID == "python-setup-py" {
					parser = detector.Analyzer
				}
			}
			if parser == nil {
				t.Fatal("missing setup.py analyzer")
			}
			result, err := parser.Analyze("setup.py", []byte("from setuptools import setup\nsetup("+test.args+")\n"))
			if err != nil {
				t.Fatal(err)
			}
			if !result.Recognized || result.Analysis != (SourceAnalysis{Presence: test.presence, Extraction: test.extraction}) {
				t.Fatalf("unexpected analysis: %+v", result)
			}
			if got := dependencyNames(result.Dependencies); !slices.Equal(got, test.want) {
				t.Fatalf("dependencies = %v, want %v", got, test.want)
			}
			if test.extraction == ExtractionPartial || test.extraction == ExtractionFailed {
				if len(result.Diagnostics) == 0 || !strings.Contains(result.Diagnostics[0].Message, "_require") {
					t.Fatalf("missing unresolved keyword: %+v", result.Diagnostics)
				}
			}
			if err := validateSourceAnalyzerResult(result); err != nil {
				t.Fatalf("invalid result: %v", err)
			}
		})
	}
}
