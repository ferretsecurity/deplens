package main

import (
	"context"
	"testing"
)

func TestGitHubAcquisitionDeduplicatesBeforeInspectionAndPinsEvidence(t *testing.T) {
	service := &fakeGitHubService{
		searches: map[string][]GitHubCodeHit{
			"one": {{Repository: "octo/example", Path: "nested/go.mod"}},
			"two": {{Repository: "octo/example", Path: "nested/go.mod"}, {Repository: "octo/other", Path: "go.mod"}},
		},
		repositories: map[string]GitHubRepository{
			"octo/example": {FullName: "octo/example", HTMLURL: "https://github.com/octo/example", DefaultBranch: "main"},
			"octo/other":   {FullName: "octo/other", HTMLURL: "https://github.com/octo/other", DefaultBranch: "main"},
		},
		heads: map[string]string{"octo/example/main": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "octo/other/main": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
		files: map[string][]byte{
			"octo/example@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa:nested/go.mod": []byte("module example\n"),
			"octo/example@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa:LICENSE":       []byte("MIT License\n"),
			"octo/other@bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb:go.mod":          []byte("module other\n"),
			"octo/other@bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb:LICENSE":         []byte("MIT License\n"),
		},
	}
	acquisition := newGitHubAcquisition(service, CollectionLimits{Queries: 2, ResultPages: 2, CandidateInspections: 2, DecodedResponseBytes: 4096, SourceBytes: 1024})
	input, err := acquisition.Acquire(context.Background(), Iteration{QueryPlan: []string{"one", "two"}, QueryLimit: 2, CandidateLimit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(input.Candidates) != 2 || service.repositoryCalls != 2 {
		t.Fatalf("candidates=%d inspections=%d", len(input.Candidates), service.repositoryCalls)
	}
	if got, want := input.Outcome.Queries, []string{"one", "two"}; !sameStrings(got, want) {
		t.Fatalf("queries=%q want %q", got, want)
	}
	if input.Candidates[0].Commit != service.heads["octo/example/main"] || input.Candidates[0].License.Permalink != "https://github.com/octo/example/blob/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/LICENSE" {
		t.Fatalf("candidate not pinned: %#v", input.Candidates[0])
	}
	if service.clones != 0 || service.executions != 0 {
		t.Fatal("acquisition must not clone or execute upstream code")
	}
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

type fakeGitHubService struct {
	searches                            map[string][]GitHubCodeHit
	repositories                        map[string]GitHubRepository
	heads                               map[string]string
	files                               map[string][]byte
	repositoryCalls, clones, executions int
}

func (f *fakeGitHubService) SearchCode(_ context.Context, query string, _ int, _ int64) ([]GitHubCodeHit, bool, int64, error) {
	return f.searches[query], false, int64(len(query)), nil
}
func (f *fakeGitHubService) Repository(_ context.Context, repository string, _ int64) (GitHubRepository, int64, error) {
	f.repositoryCalls++
	return f.repositories[repository], 10, nil
}
func (f *fakeGitHubService) DefaultBranchHead(_ context.Context, repository, branch string, _ int64) (string, int64, error) {
	return f.heads[repository+"/"+branch], 10, nil
}
func (f *fakeGitHubService) File(_ context.Context, repository, commit, path string, _ int64) ([]byte, int64, error) {
	b, ok := f.files[repository+"@"+commit+":"+path]
	if !ok {
		return nil, 1, errGitHubNotFound
	}
	return b, int64(len(b)), nil
}
