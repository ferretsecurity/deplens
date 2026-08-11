package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"
)

var errGitHubNotFound = errors.New("GitHub resource not found")

var licenseFilenames = []string{"LICENSE", "LICENSE.md", "LICENSE.txt"}

// githubService is the complete remote boundary. Its byteLimit must be obeyed
// before a response is decoded, so callers can enforce their aggregate budget.
type githubService interface {
	SearchCode(context.Context, string, int, int64) ([]GitHubCodeHit, bool, int64, error)
	Repository(context.Context, string, int64) (GitHubRepository, int64, error)
	DefaultBranchHead(context.Context, string, string, int64) (string, int64, error)
	File(context.Context, string, string, string, int64) ([]byte, int64, error)
}

type GitHubCodeHit struct{ Repository, Path string }
type GitHubRepository struct {
	FullName, HTMLURL, DefaultBranch string
	Private, Fork, Template          bool
}

type githubAcquisition struct {
	service githubService
	limits  CollectionLimits
	now     func() time.Time
}

func newGitHubAcquisition(service githubService, limits CollectionLimits) *githubAcquisition {
	return &githubAcquisition{service: service, limits: limits, now: time.Now}
}

func newDefaultGitHubAcquisition(limits CollectionLimits) *githubAcquisition {
	return &githubAcquisition{limits: limits, now: time.Now}
}

func (a *githubAcquisition) ensureService() error {
	if a.service != nil {
		return nil
	}
	token, err := githubToken()
	if err != nil {
		return err
	}
	a.service = newGitHubHTTPService(token)
	return nil
}

type acquisitionBudget struct {
	limits                      CollectionLimits
	queries, pages, inspections int
	bytes                       int64
}

type acquisitionCache struct {
	repositories map[string]GitHubRepository
	heads        map[string]string
}

func newAcquisitionCache() acquisitionCache {
	return acquisitionCache{
		repositories: make(map[string]GitHubRepository),
		heads:        make(map[string]string),
	}
}

func (b *acquisitionBudget) reserveQueries() error {
	if b.queries >= b.limits.Queries {
		return errors.New("GitHub query budget exhausted")
	}
	b.queries++
	return nil
}
func (b *acquisitionBudget) reservePage() error {
	if b.pages >= b.limits.ResultPages {
		return errors.New("GitHub result-page budget exhausted")
	}
	b.pages++
	return nil
}
func (b *acquisitionBudget) reserveInspection() error {
	if b.inspections >= b.limits.CandidateInspections {
		return errors.New("GitHub inspection budget exhausted")
	}
	b.inspections++
	return nil
}
func (b *acquisitionBudget) remainingBytes() int64 {
	return int64(b.limits.DecodedResponseBytes) - b.bytes
}
func (b *acquisitionBudget) chargeBytes(n int64) error {
	if n < 0 || n > b.remainingBytes() {
		return errors.New("GitHub decoded-response byte budget exhausted")
	}
	b.bytes += n
	return nil
}

func (a *githubAcquisition) Acquire(ctx context.Context, iteration Iteration) (ResearchInput, error) {
	if err := a.ensureService(); err != nil {
		return ResearchInput{}, githubInfrastructureError(err)
	}
	b := acquisitionBudget{limits: a.limits}
	if b.limits.Queries == 0 {
		b.limits.Queries = iteration.QueryLimit
	}
	if b.limits.CandidateInspections == 0 {
		b.limits.CandidateInspections = iteration.CandidateLimit
	}
	if b.limits.ResultPages == 0 {
		b.limits.ResultPages = iteration.QueryLimit
	}
	if b.limits.DecodedResponseBytes <= 0 || b.limits.SourceBytes <= 0 {
		return ResearchInput{}, errors.New("invalid GitHub acquisition byte limits")
	}
	queryLimit := minPositive(iteration.QueryLimit, b.limits.Queries)
	candidateLimit := minPositive(iteration.CandidateLimit, b.limits.CandidateInspections)
	if len(iteration.QueryPlan) == 0 {
		return ResearchInput{Outcome: Outcome{Result: "unsuccessful"}}, nil
	}
	seen := make(map[string]struct{})
	cache := newAcquisitionCache()
	var hits []GitHubCodeHit
	outcome := Outcome{Result: "unsuccessful"}
	for _, query := range iteration.QueryPlan {
		if len(outcome.Queries) >= queryLimit || b.reserveQueries() != nil {
			break
		}
		outcome.Queries = append(outcome.Queries, query)
		for page := 1; ; page++ {
			if err := b.reservePage(); err != nil {
				break
			}
			items, next, size, err := a.service.SearchCode(ctx, query, page, b.remainingBytes())
			if err != nil {
				return ResearchInput{}, githubInfrastructureError(err)
			}
			if err := b.chargeBytes(size); err != nil {
				return ResearchInput{}, err
			}
			for _, hit := range items {
				key := strings.ToLower(hit.Repository) + "\x00" + path.Clean(hit.Path)
				if hit.Repository != "" && hit.Path != "" {
					if _, duplicate := seen[key]; duplicate {
						continue
					}
					seen[key] = struct{}{}
					hits = append(hits, hit)
				}
			}
			if !next {
				break
			}
		}
	}
	candidates := make([]SourceCandidate, 0, candidateLimit)
	inspected := 0
	for _, hit := range hits {
		if inspected >= candidateLimit || b.reserveInspection() != nil {
			break
		}
		inspected++
		candidate, reason, err := a.inspect(ctx, &b, &cache, hit)
		if err != nil {
			return ResearchInput{}, err
		}
		if reason != "" {
			outcome.Rejections = append(outcome.Rejections, hit.Repository+":"+hit.Path+":"+reason)
			continue
		}
		outcome.Candidates = append(outcome.Candidates, candidate.ID)
		candidates = append(candidates, candidate)
	}
	return ResearchInput{Candidates: candidates, Outcome: outcome}, nil
}

func (a *githubAcquisition) inspect(ctx context.Context, b *acquisitionBudget, cache *acquisitionCache, hit GitHubCodeHit) (SourceCandidate, string, error) {
	repo, ok := cache.repositories[hit.Repository]
	if !ok {
		var n int64
		var err error
		repo, n, err = a.service.Repository(ctx, hit.Repository, b.remainingBytes())
		if err != nil {
			return SourceCandidate{}, "", githubInfrastructureError(err)
		}
		if err := b.chargeBytes(n); err != nil {
			return SourceCandidate{}, "", err
		}
		cache.repositories[hit.Repository] = repo
	}
	if repo.Private || repo.Fork || repo.Template || repo.DefaultBranch == "" {
		return SourceCandidate{}, "repository-ineligible", nil
	}
	headKey := hit.Repository + "\x00" + repo.DefaultBranch
	commit, ok := cache.heads[headKey]
	if !ok {
		var n int64
		var err error
		commit, n, err = a.service.DefaultBranchHead(ctx, hit.Repository, repo.DefaultBranch, b.remainingBytes())
		if err != nil {
			return SourceCandidate{}, "", githubInfrastructureError(err)
		}
		if err := b.chargeBytes(n); err != nil {
			return SourceCandidate{}, "", err
		}
		cache.heads[headKey] = commit
	}
	if len(commit) != 40 {
		return SourceCandidate{}, "invalid-default-branch-head", nil
	}
	source, byteCount, err := a.service.File(ctx, hit.Repository, commit, hit.Path, b.remainingBytes())
	if err != nil {
		if errors.Is(err, errGitHubNotFound) {
			return SourceCandidate{}, "source-not-found", nil
		}
		return SourceCandidate{}, "", githubInfrastructureError(err)
	}
	if err := b.chargeBytes(byteCount); err != nil {
		return SourceCandidate{}, "", err
	}
	if len(source) == 0 || len(source) > a.limits.SourceBytes {
		return SourceCandidate{}, "source-size", nil
	}
	licensePath, license, err := a.license(ctx, b, hit.Repository, commit, hit.Path)
	if err != nil {
		if errors.Is(err, errGitHubNotFound) {
			return SourceCandidate{}, "license-not-found", nil
		}
		return SourceCandidate{}, "", err
	}
	spdx := detectSPDX(license)
	if !approvedLicenses[spdx] {
		return SourceCandidate{}, "license-unapproved", nil
	}
	sourceHash := sha256.Sum256(source)
	licenseHash := sha256.Sum256(license)
	c := SourceCandidate{
		Provider:      "github",
		Repository:    repo.FullName,
		RepositoryURL: repo.HTMLURL,
		DefaultBranch: repo.DefaultBranch,
		Commit:        commit,
		OriginalPath:  hit.Path,
		RetrievedAt:   a.now().UTC().Format(time.RFC3339),
		Source:        source,
		SourceSHA256:  fmt.Sprintf("%x", sourceHash),
		License: LicenseEvidence{
			SPDX:      spdx,
			Path:      licensePath,
			Permalink: repo.HTMLURL + "/blob/" + commit + "/" + licensePath,
			SHA256:    fmt.Sprintf("%x", licenseHash),
			Bytes:     license,
		},
	}
	c.ID = stableCandidateID(c.Provider, c.Repository, c.Commit, c.OriginalPath)
	return c, "", nil
}

func (a *githubAcquisition) license(ctx context.Context, b *acquisitionBudget, repository, commit, sourcePath string) (string, []byte, error) {
	for dir := path.Dir(sourcePath); ; dir = path.Dir(dir) {
		for _, name := range licenseFilenames {
			p := name
			if dir != "." {
				p = dir + "/" + name
			}
			contents, n, err := a.service.File(ctx, repository, commit, p, b.remainingBytes())
			if err == nil {
				if err := b.chargeBytes(n); err != nil {
					return "", nil, err
				}
				return p, contents, nil
			}
			if err := b.chargeBytes(n); err != nil {
				return "", nil, err
			}
			if !errors.Is(err, errGitHubNotFound) {
				return "", nil, githubInfrastructureError(err)
			}
		}
		if dir == "." {
			break
		}
	}
	return "", nil, errGitHubNotFound
}

func detectSPDX(contents []byte) string {
	s := strings.ToLower(string(contents))
	for id := range approvedLicenses {
		if strings.Contains(s, strings.ToLower(id)) || (id == "MIT" && strings.Contains(s, "mit license")) {
			return id
		}
	}
	return ""
}
func minPositive(a, b int) int {
	if a <= 0 {
		return b
	}
	if b <= 0 || a < b {
		return a
	}
	return b
}
func githubInfrastructureError(err error) error {
	return fmt.Errorf("GitHub provider failure: %w", err)
}

// githubHTTPService performs direct API requests. The token is held only in
// memory and is never returned to callers or included in errors.
type githubHTTPService struct {
	client  *http.Client
	token   string
	baseURL string
}

func newGitHubHTTPService(token string) *githubHTTPService {
	return &githubHTTPService{client: http.DefaultClient, token: token, baseURL: "https://api.github.com"}
}
func (s *githubHTTPService) request(ctx context.Context, endpoint string, limit int64, target any) (int64, error) {
	if limit < 1 {
		return 0, errors.New("GitHub decoded-response byte budget exhausted")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+endpoint, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}
	response, err := s.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	n := int64(len(raw))
	if err != nil {
		return n, err
	}
	if n > limit {
		return n, errors.New("GitHub decoded-response byte budget exhausted")
	}
	if response.StatusCode == http.StatusNotFound {
		return n, errGitHubNotFound
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return n, fmt.Errorf("GitHub HTTP status %d", response.StatusCode)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return n, err
	}
	return n, nil
}
func (s *githubHTTPService) SearchCode(ctx context.Context, query string, page int, limit int64) ([]GitHubCodeHit, bool, int64, error) {
	var response struct {
		Items []struct {
			Path       string `json:"path"`
			Repository struct {
				FullName string `json:"full_name"`
			} `json:"repository"`
		} `json:"items"`
	}
	n, err := s.request(ctx, "/search/code?q="+url.QueryEscape(query)+"&page="+fmt.Sprint(page)+"&per_page=100", limit, &response)
	hits := make([]GitHubCodeHit, len(response.Items))
	for i, item := range response.Items {
		hits[i] = GitHubCodeHit{Repository: item.Repository.FullName, Path: item.Path}
	}
	return hits, len(response.Items) == 100, n, err
}
func (s *githubHTTPService) Repository(ctx context.Context, repository string, limit int64) (GitHubRepository, int64, error) {
	var r struct {
		FullName      string `json:"full_name"`
		HTMLURL       string `json:"html_url"`
		DefaultBranch string `json:"default_branch"`
		Private       bool   `json:"private"`
		Fork          bool   `json:"fork"`
		Template      bool   `json:"is_template"`
	}
	n, err := s.request(ctx, "/repos/"+repository, limit, &r)
	return GitHubRepository{FullName: r.FullName, HTMLURL: r.HTMLURL, DefaultBranch: r.DefaultBranch, Private: r.Private, Fork: r.Fork, Template: r.Template}, n, err
}
func (s *githubHTTPService) DefaultBranchHead(ctx context.Context, repository, branch string, limit int64) (string, int64, error) {
	var r struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	n, err := s.request(ctx, "/repos/"+repository+"/git/ref/heads/"+url.PathEscape(branch), limit, &r)
	return r.Object.SHA, n, err
}
func (s *githubHTTPService) File(ctx context.Context, repository, commit, filePath string, limit int64) ([]byte, int64, error) {
	var r struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	n, err := s.request(ctx, "/repos/"+repository+"/contents/"+url.PathEscape(filePath)+"?ref="+url.QueryEscape(commit), limit, &r)
	if err != nil {
		return nil, n, err
	}
	if r.Encoding != "base64" {
		return nil, n, errors.New("unexpected GitHub file encoding")
	}
	contents, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(r.Content, "\n", ""))
	return contents, n, err
}

func githubToken() (string, error) {
	for _, key := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value, nil
		}
	}
	output, err := exec.Command("gh", "auth", "token", "--hostname", "github.com").Output()
	if err != nil {
		return "", errors.New("GitHub authentication unavailable")
	}
	token := strings.TrimSpace(string(output))
	if token == "" {
		return "", errors.New("GitHub authentication unavailable")
	}
	return token, nil
}
func (a *githubAcquisition) Preflight() error { return a.ensureService() }
