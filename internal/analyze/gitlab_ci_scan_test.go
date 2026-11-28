package analyze

import (
	"path/filepath"
	"testing"
)

func TestGitlabCIScan(t *testing.T) {
	result, err := Scan(filepath.Join("..", "..", "testdata", "gitlab-ci"), nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		path       string
		detector   DetectorID
		extraction ExtractionState
		want       map[string]string
	}{
		{".gitlab-ci.yml", "gitlab-ci", ExtractionComplete, map[string]string{"build.image": "alpine:3.22", "default.image.name": "ruby:3.4", "default.services[0]": "redis:7", "default.services[1].name": "postgres:19.0"}},
		{".gitlab/doxygen.yml", "gitlab-ci-fragment", ExtractionComplete, map[string]string{"build_doxygen_docker.image": "docker:24.0.5-cli", "build_doxygen_docker.services[0]": "docker:24.0.5-dind"}},
		{".gitlab/ci/jobs.gitlab-ci.yml", "gitlab-ci", ExtractionPartial, map[string]string{".with-postgres.services[0].name": "postgres:19.0"}},
		{"build/build.gitlab-ci.yml", "gitlab-ci", ExtractionComplete, map[string]string{"image": "docker:dind", "services[0]": "docker:dind"}},
	}
	for _, tt := range cases {
		t.Run(tt.path, func(t *testing.T) {
			source := sourceForPath(t, result, tt.path)
			if source.Detector != tt.detector || source.Analysis.Presence != PresencePresent || source.Analysis.Extraction != tt.extraction {
				t.Fatalf("source = %+v", source)
			}
			if len(source.Dependencies) != len(tt.want) {
				t.Fatalf("dependencies = %+v", source.Dependencies)
			}
			for _, dep := range source.Dependencies {
				if tt.want[dep.SourceGroup] != dep.Raw || dep.PackageType != "docker" || dep.Relationship != RelationshipDirect || dep.Scope != ScopeRuntime {
					t.Fatalf("dependency = %+v", dep)
				}
			}
		})
	}
}
