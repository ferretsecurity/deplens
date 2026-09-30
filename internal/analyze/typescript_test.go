package analyze

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestTypeScriptConfiguredModuleCompatibility(t *testing.T) {
	for _, module := range []string{"aws-cdk-lib/aws-glue", "custom/jobs"} {
		t.Run(module, func(t *testing.T) {
			index := 2
			matcher, err := newTypeScriptMatcher(typescriptMatcherConfig{CDKConstruct: &typescriptCDKConstructConfig{
				Module: module, Construct: "CfnJob", PropsArgumentIndex: &index,
				Within:     []string{"defaultArguments"},
				Conditions: []typescriptObjectConditionConfig{{Key: "--additional-python-modules", Present: true}},
				Extract:    &typescriptExtractConfig{Key: "--additional-python-modules", Split: "comma"},
			}})
			if err != nil {
				t.Fatal(err)
			}
			body := `
new Job(this, "Job", { defaultArguments: { "--additional-python-modules": "pandas==2.2.1" } });`
			canonical := `import { CfnJob as Job } from "` + module + `";` + body
			want, err := matcher.Analyze("job.ts", []byte(canonical))
			if err != nil {
				t.Fatal(err)
			}
			if !want.Recognized || want.Analysis.Extraction != ExtractionComplete || len(want.Dependencies) != 1 || want.Dependencies[0].Name != "pandas" || len(want.Groups) != 1 || want.Groups[0].State != GroupReady {
				t.Fatalf("missing canonical dependencies/groups: %+v", want)
			}
			cases := map[string]string{
				"named alias":               canonical,
				"namespace alias":           `import * as cdk from "` + module + `";` + strings.Replace(body, "new Job", "new cdk.CfnJob", 1),
				"unicode module":            `import { CfnJob as Job } from "` + fmt.Sprintf(`\u%04x`, module[0]) + module[1:] + `";` + body,
				"hex module":                `import { CfnJob as Job } from "` + fmt.Sprintf(`\x%02x`, module[0]) + module[1:] + `";` + body,
				"malformed trailing source": canonical + "\nfunction broken( {",
			}
			for name, code := range cases {
				t.Run(name, func(t *testing.T) {
					got, err := matcher.Analyze("job.ts", []byte(code))
					if err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("got %+v, want %+v", got, want)
					}
				})
			}
			for name, code := range map[string]string{
				"ordinary":           "export function square(x: number) { return x*x; }",
				"ordinary malformed": "function broken( {",
				"comment module":     `// import { CfnJob as Job } from "` + module + `";` + body,
				"string module":      `const text = 'import { CfnJob as Job } from "` + module + `";';` + body,
				"different import":   `import { CfnJob as Job } from "other/jobs";` + body,
				"unrelated escape":   `const text = "\n";` + body,
			} {
				t.Run(name, func(t *testing.T) {
					got, err := matcher.Analyze("job.ts", []byte(code))
					if err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(got, sourceAnalyzerResult{}) {
						t.Fatalf("unexpected recognition: %+v", got)
					}
				})
			}
			dynamic := strings.Replace(canonical, `"pandas==2.2.1"`, "getModules()", 1)
			got, err := matcher.Analyze("job.ts", []byte(dynamic))
			if err != nil {
				t.Fatal(err)
			}
			if !got.Recognized || got.Analysis != failedAnalysis() || len(got.Diagnostics) != 1 || got.Diagnostics[0].Code != "typescript-cdk-incomplete" || !strings.Contains(got.Diagnostics[0].Message, "cannot be evaluated statically") {
				t.Fatalf("missing incomplete declaration diagnostic: %+v", got)
			}
		})
	}
}
