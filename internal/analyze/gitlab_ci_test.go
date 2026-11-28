package analyze

import (
	"strings"
	"testing"
)

func TestGitlabCI(t *testing.T) {
	cases := []struct {
		name, content      string
		count, diagnostics int
		extraction         ExtractionState
	}{
		{"component header", "spec: {inputs: {}}\n---\ntest: {image: alpine:3}\n", 1, 0, ExtractionComplete},
		{"empty", "stages: [test]\n", 0, 0, ExtractionComplete},
		{"reserved keys", "variables: {image: alpine:3, services: [redis:7]}\nworkflow: {image: ignored:1}\n", 0, 0, ExtractionComplete},
		{"job mapping and digest", "test:\n  image: {name: 'registry.example:5000/team/tool:1@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef'}\n  services: [{name: 'redis:7', alias: cache}]\n", 2, 0, ExtractionComplete},
		{"anchors", ".base: &base\n  image: alpine:3\ntest:\n  <<: *base\n", 2, 0, ExtractionComplete},
		{"dynamic", "image: $IMAGE\nservices: [\"docker:${VERSION}-dind\"]\n", 0, 2, ExtractionUnsupported},
		{"reference tag", "test:\n  image: !reference [.base, image]\n", 0, 1, ExtractionUnsupported},
		{"partial", "image: alpine:3\nservices: [{alias: db}, 123, 'bad image', 'UPPER:tag']\n", 1, 4, ExtractionPartial},
		{"bad service list", "services: {name: postgres:19}\n", 0, 1, ExtractionUnsupported},
		{"null overrides", "image: null\nservices: null\ntest: {image: null, services: []}\n", 0, 0, ExtractionComplete},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			result, err := (gitlabCIParser{}).Analyze(".gitlab-ci.yml", []byte(tt.content))
			if err != nil {
				t.Fatal(err)
			}
			if tt.name == "empty" || tt.name == "reserved keys" {
				if result.Recognized {
					t.Fatalf("unexpected recognition: %+v", result)
				}
				return
			}
			if !result.Recognized || result.Analysis.Extraction != tt.extraction || len(result.Dependencies) != tt.count || len(result.Diagnostics) != tt.diagnostics {
				t.Fatalf("result = %+v", result)
			}
			for _, dep := range result.Dependencies {
				if strings.Contains(dep.Raw, "@sha256:") && (dep.Name != "registry.example:5000/team/tool" || dep.Version != "1" || dep.Attributes["digest"] != "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef") {
					t.Fatalf("digest = %+v", dep)
				}
			}
		})
	}
}

func TestGitlabCIMalformed(t *testing.T) {
	if _, err := (gitlabCIParser{}).Analyze(".gitlab-ci.yml", []byte("job: [")); err == nil {
		t.Fatal("expected syntax error")
	}
}
