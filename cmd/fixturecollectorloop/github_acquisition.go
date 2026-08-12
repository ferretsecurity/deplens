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
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	errGitHubNotFound                       = errors.New("GitHub resource not found")
	errGitHubDecodedResponseBudgetExhausted = errors.New("GitHub decoded-response byte budget exhausted")
)

var licenseFilenames = []string{"LICENSE", "LICENSE.md", "LICENSE.txt"}

const (
	packetTokenBytes  = 4
	packetFramingSize = 512
)

// qualificationReason is deliberately closed: progress stores only one of
// these stable codes, never upstream response text or rejected payload bytes.
type qualificationReason string

const (
	reasonRepositoryPrivate  qualificationReason = "repository-private"
	reasonRepositoryFork     qualificationReason = "repository-fork"
	reasonRepositoryTemplate qualificationReason = "repository-template"
	reasonRepositoryPurpose  qualificationReason = "repository-purpose"
	reasonDefaultBranch      qualificationReason = "default-branch-unavailable"
	reasonDefaultBranchHead  qualificationReason = "invalid-default-branch-head"
	reasonSourceNotFound     qualificationReason = "source-not-found"
	reasonSourcePath         qualificationReason = "source-path-invalid"
	reasonSourceNotRegular   qualificationReason = "source-not-regular-file"
	reasonSourceSelector     qualificationReason = "source-selector-mismatch"
	reasonSourceSize         qualificationReason = "source-size-ceiling"
	reasonSourceUTF8         qualificationReason = "not-model-presentable-non-utf8"
	reasonSourcePacketSize   qualificationReason = "not-model-presentable-packet-size"
	reasonSourceLFS          qualificationReason = "source-lfs-pointer"
	reasonSensitiveContent   qualificationReason = "sensitive-content"
	reasonLicenseMissing     qualificationReason = "license-missing"
	reasonLicenseDisallowed  qualificationReason = "license-disallowed"
	reasonLicenseConflicting qualificationReason = "license-conflicting"
	reasonLicenseAmbiguous   qualificationReason = "license-ambiguous"
	reasonDuplicateIdentity  qualificationReason = "duplicate-identity"
	reasonDuplicateContent   qualificationReason = "duplicate-content"
)

// githubService is the complete remote boundary. Its byteLimit must be obeyed
// before a response is decoded, so callers can enforce their aggregate budget.
type githubService interface {
	SearchCode(context.Context, string, int, int64) ([]GitHubCodeHit, bool, int64, error)
	Repository(context.Context, string, int64) (GitHubRepository, int64, error)
	DefaultBranchHead(context.Context, string, string, int64) (string, int64, error)
	File(context.Context, string, string, string, int64) ([]byte, int64, error)
}

// webHintService is intentionally narrower than githubService: its results
// are untrusted coordinates only. Acquisition always re-fetches repository,
// commit, source, and license evidence through githubService.
type webHintService interface {
	SearchHints(context.Context, string, int, int64) ([]GitHubCodeHit, bool, int64, error)
}

type hitSearch func(context.Context, string, int, int64) ([]GitHubCodeHit, bool, int64, error)

// githubFileTypeService is kept separate so acquisition fakes that only model
// bytes remain useful. The production service always supplies this evidence;
// an absent implementation is treated as the legacy fixture convention of a
// regular file, never as a signal from upstream.
type githubFileTypeService interface {
	FileType(context.Context, string, string, string, int64) (string, int64, error)
}

// githubSourceService lets production adapters return regular-file evidence
// and exact bytes from one provider response. Adapters without it retain the
// narrower FileType/File seam used by lightweight tests.
type githubSourceService interface {
	Source(context.Context, string, string, string, int64) (string, []byte, int64, error)
}

type GitHubCodeHit struct{ Repository, Path string }
type GitHubRepository struct {
	FullName, HTMLURL, DefaultBranch string
	Private, Fork, Template          bool
	Description                      string
	Topics                           []string
}

type githubAcquisition struct {
	service githubService
	web     webHintService
	limits  CollectionLimits
	now     func() time.Time
}

func newGitHubAcquisition(service githubService, limits CollectionLimits) *githubAcquisition {
	return &githubAcquisition{service: service, limits: limits, now: time.Now}
}

func newGitHubAcquisitionWithWebHints(service githubService, web webHintService, limits CollectionLimits) *githubAcquisition {
	return &githubAcquisition{service: service, web: web, limits: limits, now: time.Now}
}

func newDefaultGitHubAcquisition(limits CollectionLimits) *githubAcquisition {
	return &githubAcquisition{web: newConfiguredWebHintService(), limits: limits, now: time.Now}
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
	searchLimit, searchBytes    int64
	acquisitionLimit            int64
	acquisitionBytes            int64
}

const searchResponseBudgetDivisor = 4

func newAcquisitionBudget(limits CollectionLimits) acquisitionBudget {
	total := int64(limits.DecodedResponseBytes)
	searchLimit := total / searchResponseBudgetDivisor
	if searchLimit < 1 && total > 0 {
		searchLimit = 1
	}
	return acquisitionBudget{
		limits: limits, searchLimit: searchLimit,
		acquisitionLimit: total - searchLimit,
	}
}

type acquisitionCache struct {
	repositories map[string]GitHubRepository
	heads        map[string]string
}

type hitCollector struct {
	hits             []GitHubCodeHit
	seen             map[string]struct{}
	discoveringQuery map[string]string
}

func newHitCollector() *hitCollector {
	return &hitCollector{
		seen:             make(map[string]struct{}),
		discoveringQuery: make(map[string]string),
	}
}

func (c *hitCollector) add(items []GitHubCodeHit, query string) {
	for _, hit := range items {
		if hit.Repository == "" || hit.Path == "" {
			continue
		}
		key := hitKey(hit)
		if _, duplicate := c.seen[key]; duplicate {
			continue
		}
		c.seen[key] = struct{}{}
		c.hits = append(c.hits, hit)
		c.discoveringQuery[key] = query
	}
}

func hitKey(hit GitHubCodeHit) string {
	return strings.ToLower(hit.Repository) + "\x00" + path.Clean(hit.Path)
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
func (b *acquisitionBudget) recordInspection() {
	b.inspections++
}
func (b *acquisitionBudget) remainingSearchBytes() int64 {
	return b.searchLimit - b.searchBytes
}
func (b *acquisitionBudget) remainingAcquisitionBytes() int64 {
	return b.acquisitionLimit - b.acquisitionBytes
}
func (b *acquisitionBudget) chargeSearchBytes(n int64) error {
	if n < 0 || n > b.remainingSearchBytes() {
		return errors.New("GitHub search decoded-response byte budget exhausted")
	}
	b.searchBytes += n
	return nil
}
func (b *acquisitionBudget) chargeAcquisitionBytes(n int64) error {
	if n < 0 {
		return errors.New("GitHub acquisition returned a negative decoded-response byte count")
	}
	if n > b.remainingAcquisitionBytes() {
		b.acquisitionBytes = b.acquisitionLimit
		return fmt.Errorf("GitHub acquisition decoded-response byte budget exhausted: %w", errGitHubDecodedResponseBudgetExhausted)
	}
	b.acquisitionBytes += n
	return nil
}

func (a *githubAcquisition) Acquire(ctx context.Context, iteration Iteration) (ResearchInput, error) {
	if err := a.ensureService(); err != nil {
		return ResearchInput{}, githubInfrastructureError(err)
	}
	limits := a.limits
	if limits.Queries == 0 {
		limits.Queries = iteration.QueryLimit
	}
	if limits.CandidateInspections == 0 {
		limits.CandidateInspections = iteration.CandidateLimit
	}
	if limits.ResultPages == 0 {
		limits.ResultPages = iteration.QueryLimit
	}
	if limits.DecodedResponseBytes <= 0 || limits.SourceBytes <= 0 {
		return ResearchInput{}, errors.New("invalid GitHub acquisition byte limits")
	}
	b := newAcquisitionBudget(limits)
	queryLimit := minPositive(iteration.QueryLimit, limits.Queries)
	inspectionTarget := minPositive(iteration.CandidateLimit, limits.CandidateInspections)
	if len(iteration.QueryPlan) == 0 {
		return ResearchInput{Outcome: Outcome{Result: "unsuccessful"}}, nil
	}
	cache := newAcquisitionCache()
	hits := newHitCollector()
	outcome := Outcome{Result: "unsuccessful"}
	candidates := make([]SourceCandidate, 0, inspectionTarget)
	identities, contents := make(map[string]struct{}), make(map[string]struct{})
	inspected := 0
	rejected := 0
	acquisitionBudgetExhausted := false
	reportEvery := max(1, (inspectionTarget+9)/10)
	lastReported := -1
	reportQualification := func(final bool) {
		if iteration.ReportProgress == nil {
			return
		}
		if !final && inspected != 1 && inspected%reportEvery != 0 {
			return
		}
		if !final && inspected == lastReported {
			return
		}
		lastReported = inspected
		filtered := 0
		for _, count := range outcome.FilteredSearchHits {
			filtered += count
		}
		iteration.ReportProgress(ResearchProgress{
			Stage: progressQualification, Final: final,
			Inspected: inspected, InspectionLimit: inspectionTarget,
			Qualified: len(candidates), Rejected: rejected, Filtered: filtered,
			Budget: "acquisition", DownloadedBytes: b.acquisitionBytes,
			ByteLimit: b.acquisitionLimit, RemainingBytes: b.remainingAcquisitionBytes(),
		})
	}
	qualificationComplete := func() bool {
		return inspected >= inspectionTarget && len(candidates) >= minimumQualifiedCandidates
	}
	inspectHits := func(start int) (bool, error) {
		for _, hit := range hits.hits[start:] {
			if qualificationComplete() {
				return true, nil
			}
			// GitHub search qualifiers are discovery hints, not exact
			// selectors. Discard loose matches before they consume an
			// inspection or any repository-specific request.
			if reason := qualifySourcePath(iteration.QueryPlan, hit.Path); reason != "" {
				if outcome.FilteredSearchHits == nil {
					outcome.FilteredSearchHits = make(map[string]int)
				}
				outcome.FilteredSearchHits[string(reason)]++
				continue
			}
			b.recordInspection()
			inspected++
			candidate, reason, err := a.inspect(ctx, &b, &cache, hit)
			if err != nil {
				if errors.Is(err, errGitHubDecodedResponseBudgetExhausted) {
					b.acquisitionBytes = b.acquisitionLimit
					acquisitionBudgetExhausted = true
					return true, nil
				}
				return false, err
			}
			if reason != "" {
				outcome.Rejections = append(outcome.Rejections, string(reason))
				rejected++
				reportQualification(false)
				continue
			}
			identity := candidate.Repository + "@" + candidate.Commit + ":" + candidate.OriginalPath
			if _, exists := identities[identity]; exists {
				outcome.Rejections = append(outcome.Rejections, string(reasonDuplicateIdentity))
				rejected++
				reportQualification(false)
				continue
			}
			if _, exists := contents[candidate.SourceSHA256]; exists {
				outcome.Rejections = append(outcome.Rejections, string(reasonDuplicateContent))
				rejected++
				reportQualification(false)
				continue
			}
			identities[identity], contents[candidate.SourceSHA256] = struct{}{}, struct{}{}
			candidate.DiscoveringQuery = hits.discoveringQuery[hitKey(hit)]
			outcome.Candidates = append(outcome.Candidates, candidate.ID)
			candidates = append(candidates, candidate)
			reportQualification(false)
		}
		return qualificationComplete(), nil
	}
	if _, err := collectSearchHits(ctx, &b, iteration, "github", iteration.QueryPlan, queryLimit, &outcome, hits, a.service.SearchCode, githubInfrastructureError, qualificationComplete, inspectHits); err != nil {
		return ResearchInput{}, err
	}
	if a.web != nil && !acquisitionBudgetExhausted && len(candidates) < minimumQualifiedCandidates {
		if _, err := collectSearchHits(ctx, &b, iteration, "web", iteration.QueryPlan, queryLimit, &outcome, hits, a.web.SearchHints, webInfrastructureError, qualificationComplete, inspectHits); err != nil {
			return ResearchInput{}, err
		}
	}
	reportQualification(true)
	return ResearchInput{Candidates: candidates, Outcome: outcome}, nil
}

const minimumQualifiedCandidates = 5

func collectSearchHits(ctx context.Context, budget *acquisitionBudget, iteration Iteration, provider string, queries []string, queryLimit int, outcome *Outcome, hits *hitCollector, search hitSearch, providerError func(error) error, stop func() bool, inspect func(int) (bool, error)) (bool, error) {
	remainingQueries := queryLimit - len(outcome.Queries)
	queryTotal := min(len(queries), max(0, remainingQueries))
	queryIndex := 0
	for _, query := range queries {
		if stop() {
			return true, nil
		}
		if len(outcome.Queries) >= queryLimit || budget.reserveQueries() != nil {
			break
		}
		queryIndex++
		outcome.Queries = append(outcome.Queries, query)
		for page := 1; ; page++ {
			if err := budget.reservePage(); err != nil {
				break
			}
			items, next, size, err := search(ctx, query, page, budget.remainingSearchBytes())
			if err != nil {
				return false, providerError(err)
			}
			if err := budget.chargeSearchBytes(size); err != nil {
				return false, err
			}
			firstNewHit := len(hits.hits)
			hits.add(items, query)
			if iteration.ReportProgress != nil {
				iteration.ReportProgress(ResearchProgress{
					Stage: progressSearch, Provider: provider, Query: query,
					QueryIndex: queryIndex, QueryTotal: queryTotal, Page: page, Hits: len(hits.hits),
					Budget: "search", DownloadedBytes: budget.searchBytes,
					ByteLimit: budget.searchLimit, RemainingBytes: budget.remainingSearchBytes(),
				})
			}
			stopped, err := inspect(firstNewHit)
			if err != nil {
				return false, err
			}
			if stopped {
				return true, nil
			}
			if !next {
				break
			}
		}
	}
	return stop(), nil
}

func (a *githubAcquisition) inspect(ctx context.Context, b *acquisitionBudget, cache *acquisitionCache, hit GitHubCodeHit) (SourceCandidate, qualificationReason, error) {
	repo, ok := cache.repositories[hit.Repository]
	if !ok {
		var n int64
		var err error
		repo, n, err = a.service.Repository(ctx, hit.Repository, b.remainingAcquisitionBytes())
		if err != nil {
			return SourceCandidate{}, "", githubInfrastructureError(err)
		}
		if err := b.chargeAcquisitionBytes(n); err != nil {
			return SourceCandidate{}, "", err
		}
		cache.repositories[hit.Repository] = repo
	}
	if reason := qualifyRepository(repo); reason != "" {
		return SourceCandidate{}, reason, nil
	}
	headKey := hit.Repository + "\x00" + repo.DefaultBranch
	commit, ok := cache.heads[headKey]
	if !ok {
		var n int64
		var err error
		commit, n, err = a.service.DefaultBranchHead(ctx, hit.Repository, repo.DefaultBranch, b.remainingAcquisitionBytes())
		if err != nil {
			return SourceCandidate{}, "", githubInfrastructureError(err)
		}
		if err := b.chargeAcquisitionBytes(n); err != nil {
			return SourceCandidate{}, "", err
		}
		cache.heads[headKey] = commit
	}
	if len(commit) != 40 {
		return SourceCandidate{}, reasonDefaultBranchHead, nil
	}
	var source []byte
	if combined, ok := a.service.(githubSourceService); ok {
		fileType, contents, n, err := combined.Source(ctx, hit.Repository, commit, hit.Path, b.remainingAcquisitionBytes())
		if chargeErr := b.chargeAcquisitionBytes(n); chargeErr != nil {
			return SourceCandidate{}, "", chargeErr
		}
		if err != nil {
			if errors.Is(err, errGitHubNotFound) {
				return SourceCandidate{}, reasonSourceNotFound, nil
			}
			return SourceCandidate{}, "", githubInfrastructureError(err)
		}
		if fileType != "file" {
			return SourceCandidate{}, reasonSourceNotRegular, nil
		}
		source = contents
	} else if typed, ok := a.service.(githubFileTypeService); ok {
		fileType, n, err := typed.FileType(ctx, hit.Repository, commit, hit.Path, b.remainingAcquisitionBytes())
		if chargeErr := b.chargeAcquisitionBytes(n); chargeErr != nil {
			return SourceCandidate{}, "", chargeErr
		}
		if err != nil {
			if errors.Is(err, errGitHubNotFound) {
				return SourceCandidate{}, reasonSourceNotFound, nil
			}
			return SourceCandidate{}, "", githubInfrastructureError(err)
		}
		if fileType != "file" {
			return SourceCandidate{}, reasonSourceNotRegular, nil
		}
	}
	if source == nil {
		var byteCount int64
		var err error
		source, byteCount, err = a.service.File(ctx, hit.Repository, commit, hit.Path, b.remainingAcquisitionBytes())
		if err != nil {
			if errors.Is(err, errGitHubNotFound) {
				return SourceCandidate{}, reasonSourceNotFound, nil
			}
			return SourceCandidate{}, "", githubInfrastructureError(err)
		}
		if err := b.chargeAcquisitionBytes(byteCount); err != nil {
			return SourceCandidate{}, "", err
		}
	}
	if len(source) == 0 || len(source) > a.limits.SourceBytes {
		return SourceCandidate{}, reasonSourceSize, nil
	}
	if !utf8.Valid(source) {
		return SourceCandidate{}, reasonSourceUTF8, nil
	}
	if sourceExceedsPacketCeiling(source, a.limits.PacketTokens) {
		return SourceCandidate{}, reasonSourcePacketSize, nil
	}
	content := string(source)
	if isLFSPointer(content) {
		return SourceCandidate{}, reasonSourceLFS, nil
	}
	if containsUnsafeContent(content) {
		return SourceCandidate{}, reasonSensitiveContent, nil
	}
	licensePath, license, licenseReason, err := a.license(ctx, b, hit.Repository, commit, hit.Path)
	if err != nil {
		if errors.Is(err, errGitHubNotFound) {
			return SourceCandidate{}, reasonLicenseMissing, nil
		}
		return SourceCandidate{}, "", err
	}
	if licenseReason != "" {
		return SourceCandidate{}, licenseReason, nil
	}
	spdx, ambiguous := detectSPDX(license)
	if ambiguous {
		return SourceCandidate{}, reasonLicenseAmbiguous, nil
	}
	if !approvedLicenses[spdx] {
		return SourceCandidate{}, reasonLicenseDisallowed, nil
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

// Source bytes are never truncated or transformed to force packet fit. The
// conservative per-source ceiling reserves JSON object framing inside the
// configured aggregate packet target; a zero target means this adapter has no
// model-presentation budget to enforce yet.
func sourceExceedsPacketCeiling(source []byte, packetTokens int) bool {
	if packetTokens <= 0 {
		return false
	}
	encoded, err := json.Marshal(string(source))
	return err != nil || len(encoded)+packetFramingSize > packetTokens*packetTokenBytes
}

// license resolves all recognized license-file names at a directory before
// climbing. A same-scope disagreement is never silently decided by filename.
func (a *githubAcquisition) license(ctx context.Context, b *acquisitionBudget, repository, commit, sourcePath string) (string, []byte, qualificationReason, error) {
	for dir := path.Dir(sourcePath); ; dir = path.Dir(dir) {
		type foundLicense struct {
			path  string
			bytes []byte
		}
		var found []foundLicense
		for _, name := range licenseFilenames {
			p := name
			if dir != "." {
				p = dir + "/" + name
			}
			contents, n, err := a.service.File(ctx, repository, commit, p, b.remainingAcquisitionBytes())
			if err := b.chargeAcquisitionBytes(n); err != nil {
				return "", nil, "", err
			}
			if err == nil {
				found = append(found, foundLicense{p, contents})
				continue
			}
			if !errors.Is(err, errGitHubNotFound) {
				return "", nil, "", githubInfrastructureError(err)
			}
		}
		if len(found) == 1 {
			return found[0].path, found[0].bytes, "", nil
		}
		if len(found) > 1 {
			spdx, ambiguous := detectSPDX(found[0].bytes)
			if ambiguous {
				return "", nil, reasonLicenseAmbiguous, nil
			}
			for _, item := range found[1:] {
				itemSPDX, itemAmbiguous := detectSPDX(item.bytes)
				if itemAmbiguous {
					return "", nil, reasonLicenseAmbiguous, nil
				}
				if itemSPDX != spdx {
					return "", nil, reasonLicenseConflicting, nil
				}
			}
			return "", nil, reasonLicenseAmbiguous, nil
		}
		if dir == "." {
			break
		}
	}
	return "", nil, "", errGitHubNotFound
}

func qualifyRepository(repo GitHubRepository) qualificationReason {
	switch {
	case repo.Private:
		return reasonRepositoryPrivate
	case repo.Fork:
		return reasonRepositoryFork
	case repo.Template:
		return reasonRepositoryTemplate
	case repo.DefaultBranch == "":
		return reasonDefaultBranch
	}
	purpose := strings.ToLower(repo.FullName + " " + repo.Description + " " + strings.Join(repo.Topics, " "))
	if repositoryPurposePattern.MatchString(purpose) {
		return reasonRepositoryPurpose
	}
	return ""
}

var repositoryPurposePattern = regexp.MustCompile(`\b(fixture|fixtures|demo|demos|example|examples|training[ -]?data|test[ -]?data|sample[ -]?data)\b`)

func qualifySourcePath(queries []string, sourcePath string) qualificationReason {
	clean := path.Clean(sourcePath)
	if sourcePath == "" || strings.HasPrefix(clean, "../") || clean == ".." || strings.HasPrefix(sourcePath, "/") {
		return reasonSourcePath
	}
	for _, query := range queries {
		if queryMatchesSourcePath(query, clean) {
			return ""
		}
	}
	return reasonSourceSelector
}

// Query plans are generated directly from detector selectors. This validates
// selector terms against the path only; it does not involve analyzer output.
func queryMatchesSourcePath(query, sourcePath string) bool {
	for _, term := range strings.Fields(query) {
		key, value, ok := strings.Cut(term, ":")
		if !ok {
			continue
		}
		value = strings.Trim(value, `"`)
		switch key {
		case "filename":
			if path.Base(sourcePath) != value {
				return false
			}
		case "path":
			if sourcePath != value && !strings.HasPrefix(sourcePath, strings.TrimSuffix(value, "/")+"/") {
				return false
			}
		case "extension":
			if !strings.EqualFold(path.Ext(sourcePath), "."+value) {
				return false
			}
		}
	}
	return true
}

func detectSPDX(contents []byte) (spdx string, ambiguous bool) {
	licenseText := strings.ToLower(string(contents))
	for id := range approvedLicenses {
		if !strings.Contains(licenseText, strings.ToLower(id)) && (id != "MIT" || !strings.Contains(licenseText, "mit license")) {
			continue
		}
		if spdx != "" {
			return "", true
		}
		spdx = id
	}
	return spdx, false
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

func webInfrastructureError(err error) error {
	return fmt.Errorf("web-search provider failure: %w", err)
}

// webHintHTTPService has a deliberately small, replaceable response contract:
// {"results":[{"repository":"owner/repository","path":"relative/path"}]}.
// The endpoint and token are operational acquisition configuration; neither is
// represented in progress nor supplied to the selector.
type webHintHTTPService struct {
	client   *http.Client
	endpoint string
	token    string
}

func newConfiguredWebHintService() webHintService {
	endpoint := strings.TrimSpace(os.Getenv("FIXTURE_COLLECTOR_WEB_SEARCH_ENDPOINT"))
	if endpoint == "" {
		return nil
	}
	return &webHintHTTPService{
		client: http.DefaultClient, endpoint: strings.TrimRight(endpoint, "/"),
		token: strings.TrimSpace(os.Getenv("FIXTURE_COLLECTOR_WEB_SEARCH_TOKEN")),
	}
}

func (s *webHintHTTPService) SearchHints(ctx context.Context, query string, page int, limit int64) ([]GitHubCodeHit, bool, int64, error) {
	if limit < 1 {
		return nil, false, 0, errors.New("web-search decoded-response byte budget exhausted")
	}
	requestURL, err := url.Parse(s.endpoint)
	if err != nil {
		return nil, false, 0, err
	}
	values := requestURL.Query()
	values.Set("q", query)
	values.Set("page", fmt.Sprint(page))
	requestURL.RawQuery = values.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return nil, false, 0, err
	}
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}
	response, err := s.client.Do(req)
	if err != nil {
		return nil, false, 0, err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	n := int64(len(raw))
	if err != nil {
		return nil, false, n, err
	}
	if n > limit {
		return nil, false, n, errors.New("web-search decoded-response byte budget exhausted")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode > http.StatusMultipleChoices-1 {
		return nil, false, n, fmt.Errorf("web-search HTTP status %d", response.StatusCode)
	}
	var payload struct {
		Results []GitHubCodeHit `json:"results"`
		Next    bool            `json:"next"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, false, n, err
	}
	return payload.Results, payload.Next, n, nil
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
		return 0, errGitHubDecodedResponseBudgetExhausted
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
		return n, errGitHubDecodedResponseBudgetExhausted
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
		FullName      string   `json:"full_name"`
		HTMLURL       string   `json:"html_url"`
		DefaultBranch string   `json:"default_branch"`
		Private       bool     `json:"private"`
		Fork          bool     `json:"fork"`
		Template      bool     `json:"is_template"`
		Description   string   `json:"description"`
		Topics        []string `json:"topics"`
	}
	n, err := s.request(ctx, "/repos/"+repository, limit, &r)
	return GitHubRepository{
		FullName:      r.FullName,
		HTMLURL:       r.HTMLURL,
		DefaultBranch: r.DefaultBranch,
		Private:       r.Private,
		Fork:          r.Fork,
		Template:      r.Template,
		Description:   r.Description,
		Topics:        r.Topics,
	}, n, err
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

func (s *githubHTTPService) Source(ctx context.Context, repository, commit, filePath string, limit int64) (string, []byte, int64, error) {
	var r struct {
		Type     string `json:"type"`
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	n, err := s.request(ctx, "/repos/"+repository+"/contents/"+url.PathEscape(filePath)+"?ref="+url.QueryEscape(commit), limit, &r)
	if err != nil || r.Type != "file" {
		return r.Type, nil, n, err
	}
	if r.Encoding != "base64" {
		return r.Type, nil, n, errors.New("unexpected GitHub file encoding")
	}
	contents, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(r.Content, "\n", ""))
	return r.Type, contents, n, err
}
func (s *githubHTTPService) FileType(ctx context.Context, repository, commit, filePath string, limit int64) (string, int64, error) {
	var r struct {
		Type string `json:"type"`
	}
	n, err := s.request(ctx, "/repos/"+repository+"/contents/"+url.PathEscape(filePath)+"?ref="+url.QueryEscape(commit), limit, &r)
	return r.Type, n, err
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
