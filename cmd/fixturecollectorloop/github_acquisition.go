package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
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
	githubUserAgent   = "deplens-fixture-collector"
	githubAPIVersion  = "2026-03-10"

	githubCodeSearchInterval = 6 * time.Second
	githubCodeSearchMaxPage  = 10
	githubRateLimitAttempts  = 4
	githubTransientAttempts  = 3
	githubErrorMessageLimit  = 256
	githubResetMargin        = time.Second
	githubCoreReserveDivisor = 10
	githubPacingLogThreshold = 5 * time.Second
)

type githubProgressContextKey struct{}

type githubProgressReporter struct {
	stage  string
	report func(ResearchProgress)
}

func withGitHubProgress(ctx context.Context, stage string, report func(ResearchProgress)) context.Context {
	if report == nil {
		return ctx
	}
	return context.WithValue(ctx, githubProgressContextKey{}, githubProgressReporter{stage: stage, report: report})
}

func reportGitHubProgress(ctx context.Context, resource string, event ProviderProgress) {
	reporter, ok := ctx.Value(githubProgressContextKey{}).(githubProgressReporter)
	if !ok || reporter.report == nil {
		return
	}
	stage := reporter.stage
	if stage == "" {
		stage = progressQualification
		if resource == "code_search" {
			stage = progressSearch
		}
	}
	event.Resource = resource
	reporter.report(ResearchProgress{Stage: stage, ProviderEvent: &event})
}

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

func newHitCollector(prior map[string]bool) *hitCollector {
	seen := make(map[string]struct{}, len(prior))
	for id := range prior {
		seen[id] = struct{}{}
	}
	return &hitCollector{
		seen:             seen,
		discoveringQuery: make(map[string]string),
	}
}

func (c *hitCollector) add(items []GitHubCodeHit, query string) []string {
	var addedIDs []string
	for _, hit := range items {
		if hit.Repository == "" || hit.Path == "" {
			continue
		}
		id := searchHitID(hit)
		if _, duplicate := c.seen[id]; duplicate {
			continue
		}
		c.seen[id] = struct{}{}
		c.hits = append(c.hits, hit)
		c.discoveringQuery[hitKey(hit)] = query
		addedIDs = append(addedIDs, id)
	}
	return addedIDs
}

func hitKey(hit GitHubCodeHit) string {
	return strings.ToLower(hit.Repository) + "\x00" + path.Clean(hit.Path)
}

func searchHitID(hit GitHubCodeHit) string {
	return hash("fixture-collector-search-hit-v1\x00" + hitKey(hit))
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
		return fmt.Errorf("GitHub search decoded-response byte budget exhausted: %w", errGitHubDecodedResponseBudgetExhausted)
	}
	b.searchBytes += n
	return nil
}
func (b *acquisitionBudget) exhaustSearchBytes() {
	b.searchBytes = b.searchLimit
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
	ctx = withGitHubProgress(ctx, "", iteration.ReportProgress)
	cache := newAcquisitionCache()
	hits := newHitCollector(iteration.SeenSearchHitIDs)
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
	if a.web != nil && !acquisitionBudgetExhausted && b.remainingSearchBytes() > 0 && len(candidates) < minimumQualifiedCandidates {
		if _, err := collectSearchHits(ctx, &b, iteration, "web", iteration.QueryPlan, queryLimit, &outcome, hits, a.web.SearchHints, webInfrastructureError, qualificationComplete, inspectHits); err != nil {
			return ResearchInput{}, err
		}
	}
	reportQualification(true)
	sort.Strings(outcome.SearchHitIDs)
	outcome.SearchCursors = mergeSearchCursors(nil, outcome.SearchCursors)
	providers := []string{"github"}
	if a.web != nil {
		providers = append(providers, "web")
	}
	return ResearchInput{
		Candidates: candidates, Outcome: outcome,
		SearchExhausted: allSearchQueriesExhausted(iteration.QueryPlan, providers, iteration.SearchCursors, outcome.SearchCursors),
	}, nil
}

const minimumQualifiedCandidates = 5

func collectSearchHits(ctx context.Context, budget *acquisitionBudget, iteration Iteration, provider string, queries []string, queryLimit int, outcome *Outcome, hits *hitCollector, search hitSearch, providerError func(error) error, stop func() bool, inspect func(int) (bool, error)) (bool, error) {
	remainingQueries := queryLimit - len(outcome.Queries)
	queryTotal := min(countUnexhaustedQueries(provider, queries, iteration.SearchCursors, outcome.SearchCursors), max(0, remainingQueries))
	queryIndex := 0
	for _, query := range queries {
		if stop() {
			return true, nil
		}
		cursor := effectiveSearchCursor(provider, query, iteration.SearchCursors, outcome.SearchCursors)
		if cursor.Exhausted {
			continue
		}
		if len(outcome.Queries) >= queryLimit || budget.reserveQueries() != nil {
			break
		}
		queryIndex++
		outcome.Queries = append(outcome.Queries, query)
		startPage := max(1, cursor.NextPage)
		for page := startPage; ; page++ {
			if err := budget.reservePage(); err != nil {
				setSearchCursor(&outcome.SearchCursors, SearchCursor{Provider: provider, Query: query, NextPage: page})
				return true, nil
			}
			items, next, size, err := search(ctx, query, page, budget.remainingSearchBytes())
			if err != nil {
				if errors.Is(err, errGitHubDecodedResponseBudgetExhausted) {
					setSearchCursor(&outcome.SearchCursors, SearchCursor{Provider: provider, Query: query, NextPage: page})
					budget.exhaustSearchBytes()
					reportSearchBudgetExhaustion(iteration, provider)
					return true, nil
				}
				return false, providerError(err)
			}
			if err := budget.chargeSearchBytes(size); err != nil {
				if errors.Is(err, errGitHubDecodedResponseBudgetExhausted) {
					setSearchCursor(&outcome.SearchCursors, SearchCursor{Provider: provider, Query: query, NextPage: page})
					budget.exhaustSearchBytes()
					reportSearchBudgetExhaustion(iteration, provider)
					return true, nil
				}
				return false, err
			}
			setSearchCursor(&outcome.SearchCursors, SearchCursor{Provider: provider, Query: query, NextPage: page + 1, Exhausted: !next})
			firstNewHit := len(hits.hits)
			outcome.SearchHitIDs = append(outcome.SearchHitIDs, hits.add(items, query)...)
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

func effectiveSearchCursor(provider, query string, prior, current []SearchCursor) SearchCursor {
	cursor := SearchCursor{Provider: provider, Query: query, NextPage: 1}
	for _, candidate := range prior {
		if candidate.Provider == provider && candidate.Query == query {
			cursor = candidate
		}
	}
	for _, candidate := range current {
		if candidate.Provider == provider && candidate.Query == query {
			cursor = candidate
		}
	}
	return cursor
}

func setSearchCursor(cursors *[]SearchCursor, cursor SearchCursor) {
	for i := range *cursors {
		if (*cursors)[i].Provider == cursor.Provider && (*cursors)[i].Query == cursor.Query {
			(*cursors)[i] = cursor
			return
		}
	}
	*cursors = append(*cursors, cursor)
}

func countUnexhaustedQueries(provider string, queries []string, prior, current []SearchCursor) int {
	count := 0
	for _, query := range queries {
		if !effectiveSearchCursor(provider, query, prior, current).Exhausted {
			count++
		}
	}
	return count
}

func allSearchQueriesExhausted(queries, providers []string, prior, current []SearchCursor) bool {
	for _, provider := range providers {
		for _, query := range queries {
			if !effectiveSearchCursor(provider, query, prior, current).Exhausted {
				return false
			}
		}
	}
	return len(queries) != 0
}

func reportSearchBudgetExhaustion(iteration Iteration, provider string) {
	if iteration.ReportProgress == nil {
		return
	}
	resource := provider + "_search"
	if provider == "github" {
		resource = "code_search"
	}
	iteration.ReportProgress(ResearchProgress{
		Stage: progressSearch,
		ProviderEvent: &ProviderProgress{
			Action: "stopped", Reason: "decoded-response-byte-budget", Resource: resource,
		},
	})
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
	now     func() time.Time
	sleep   func(context.Context, time.Duration) error

	gateMu        sync.Mutex
	lastRequest   map[string]time.Time
	nextRequest   map[string]time.Time
	blockedUntil  map[string]time.Time
	blockedReason map[string]string
}

func newGitHubHTTPService(token string) *githubHTTPService {
	return &githubHTTPService{
		client: http.DefaultClient, token: token, baseURL: "https://api.github.com",
		now: time.Now, sleep: contextSleep,
		lastRequest: make(map[string]time.Time), nextRequest: make(map[string]time.Time), blockedUntil: make(map[string]time.Time),
		blockedReason: make(map[string]string),
	}
}

func contextSleep(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type githubRateLimitSnapshot struct {
	status                       int
	resource, requestID, message string
	limit, remaining             int
	countersKnown                bool
	reset                        time.Time
	retryAfter                   time.Duration
	retryAfterKnown              bool
}

func githubResource(endpoint string) string {
	if strings.HasPrefix(endpoint, "/search/code") {
		return "code_search"
	}
	return "core"
}

func (s *githubHTTPService) waitForGate(ctx context.Context, resource string) error {
	now := s.now()
	s.gateMu.Lock()
	ready := now
	reason := "request-pacing"
	var reset time.Time
	if blocked := s.blockedUntil[resource]; blocked.After(ready) {
		ready = blocked
		reason = s.blockedReason[resource]
		if reason == "" {
			reason = "primary-rate-limit"
		}
		reset = blocked
	}
	if next := s.nextRequest[resource]; next.After(ready) {
		ready = next
		reason = "request-pacing"
		if resource == "core" {
			reason = "adaptive-pacing"
		}
		reset = time.Time{}
	}
	if resource == "code_search" {
		s.nextRequest[resource] = ready.Add(githubCodeSearchInterval)
	}
	s.lastRequest[resource] = ready
	s.gateMu.Unlock()
	if !ready.After(now) {
		return nil
	}
	delay := ready.Sub(now)
	if reason != "adaptive-pacing" || delay >= githubPacingLogThreshold {
		reportGitHubProgress(ctx, resource, ProviderProgress{Action: "wait", Reason: reason, Delay: delay, Reset: reset})
	}
	return s.sleep(ctx, delay)
}

func (s *githubHTTPService) rememberRateLimit(snapshot githubRateLimitSnapshot) {
	if !snapshot.countersKnown || snapshot.reset.IsZero() {
		return
	}
	now := s.now()
	if !snapshot.reset.After(now) {
		return
	}
	s.gateMu.Lock()
	defer s.gateMu.Unlock()
	reserve := 0
	if snapshot.resource == "core" && snapshot.limit > 0 {
		reserve = max(1, (snapshot.limit+githubCoreReserveDivisor-1)/githubCoreReserveDivisor)
	}
	if snapshot.remaining <= reserve {
		blocked := snapshot.reset.Add(githubResetMargin)
		if blocked.After(s.blockedUntil[snapshot.resource]) {
			s.blockedUntil[snapshot.resource] = blocked
			s.blockedReason[snapshot.resource] = "primary-rate-limit"
			if snapshot.remaining > 0 {
				s.blockedReason[snapshot.resource] = "rate-limit-reserve"
			}
		}
		return
	}
	if snapshot.resource != "core" {
		return
	}
	usableRemaining := snapshot.remaining - reserve
	interval := snapshot.reset.Sub(now) / time.Duration(usableRemaining)
	if interval > 0 {
		started := s.lastRequest[snapshot.resource]
		if started.IsZero() {
			started = now
		}
		s.nextRequest[snapshot.resource] = started.Add(interval)
	}
}

func (s *githubHTTPService) request(ctx context.Context, endpoint string, limit int64, target any) (int64, error) {
	if limit < 1 {
		return 0, errGitHubDecodedResponseBudgetExhausted
	}
	resource := githubResource(endpoint)
	var total int64
	transientAttempt := 0
	rateAttempt := 0
	for {
		if total >= limit {
			return total, errGitHubDecodedResponseBudgetExhausted
		}
		if err := s.waitForGate(ctx, resource); err != nil {
			return total, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+endpoint, nil)
		if err != nil {
			return total, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", githubUserAgent)
		req.Header.Set("X-GitHub-Api-Version", githubAPIVersion)
		if s.token != "" {
			req.Header.Set("Authorization", "Bearer "+s.token)
		}
		response, requestErr := s.client.Do(req)
		if requestErr != nil {
			if ctx.Err() != nil {
				return total, ctx.Err()
			}
			transientAttempt++
			if transientAttempt >= githubTransientAttempts || !retryableTransportError(requestErr) {
				reportGitHubProgress(ctx, resource, ProviderProgress{Action: "failure", Reason: "transport-error", Attempt: transientAttempt, MaxAttempts: githubTransientAttempts})
				return total, errors.New("GitHub transport failure")
			}
			delay := transientRetryDelay(transientAttempt)
			reportGitHubProgress(ctx, resource, ProviderProgress{Action: "wait", Reason: "transport-error", Attempt: transientAttempt, MaxAttempts: githubTransientAttempts, Delay: delay})
			if err := s.sleep(ctx, delay); err != nil {
				return total, err
			}
			continue
		}
		raw, readErr := io.ReadAll(io.LimitReader(response.Body, limit-total+1))
		response.Body.Close()
		total += int64(len(raw))
		if total > limit {
			return total, errGitHubDecodedResponseBudgetExhausted
		}
		if readErr != nil {
			if ctx.Err() != nil {
				return total, ctx.Err()
			}
			transientAttempt++
			if transientAttempt >= githubTransientAttempts || !retryableTransportError(readErr) {
				reportGitHubProgress(ctx, resource, ProviderProgress{Action: "failure", Reason: "transport-error", Attempt: transientAttempt, MaxAttempts: githubTransientAttempts})
				return total, errors.New("GitHub transport failure")
			}
			delay := transientRetryDelay(transientAttempt)
			reportGitHubProgress(ctx, resource, ProviderProgress{Action: "wait", Reason: "transport-error", Attempt: transientAttempt, MaxAttempts: githubTransientAttempts, Delay: delay})
			if err := s.sleep(ctx, delay); err != nil {
				return total, err
			}
			continue
		}
		snapshot := parseGitHubRateLimitSnapshot(response, raw, resource, s.now())
		s.rememberRateLimit(snapshot)
		if response.StatusCode == http.StatusNotFound {
			return total, errGitHubNotFound
		}
		if response.StatusCode >= 200 && response.StatusCode <= 299 {
			if err := json.Unmarshal(raw, target); err != nil {
				return total, err
			}
			return total, nil
		}

		class, retry, delay, maxAttempts := classifyGitHubResponse(snapshot, rateAttempt, transientAttempt, s.now())
		if retry {
			if class == "server-error" {
				transientAttempt++
				if transientAttempt >= maxAttempts {
					retry = false
				}
			} else {
				rateAttempt++
				if rateAttempt >= maxAttempts {
					retry = false
				}
			}
		}
		attempt := rateAttempt
		if class == "server-error" {
			attempt = transientAttempt
		}
		event := snapshot.providerProgress(class)
		event.Attempt, event.MaxAttempts, event.Delay = attempt, maxAttempts, delay
		if !retry {
			event.Action = "failure"
			event.Delay = 0
			reportGitHubProgress(ctx, snapshot.resource, event)
			return total, githubResponseError(snapshot, class)
		}
		event.Action = "wait"
		reportGitHubProgress(ctx, snapshot.resource, event)
		if err := s.sleep(ctx, delay); err != nil {
			return total, err
		}
	}
}

func parseGitHubRateLimitSnapshot(response *http.Response, raw []byte, fallbackResource string, now time.Time) githubRateLimitSnapshot {
	snapshot := githubRateLimitSnapshot{
		status: response.StatusCode, resource: sanitizeGitHubDiagnostic(response.Header.Get("X-RateLimit-Resource")),
		requestID: sanitizeGitHubDiagnostic(response.Header.Get("X-GitHub-Request-Id")),
	}
	if snapshot.resource == "" {
		snapshot.resource = fallbackResource
	}
	limit, limitOK := parseGitHubHeaderInt(response.Header.Get("X-RateLimit-Limit"))
	remaining, remainingOK := parseGitHubHeaderInt(response.Header.Get("X-RateLimit-Remaining"))
	if limitOK && remainingOK {
		snapshot.limit, snapshot.remaining, snapshot.countersKnown = limit, remaining, true
	}
	if reset, ok := parseGitHubHeaderInt64(response.Header.Get("X-RateLimit-Reset")); ok {
		snapshot.reset = time.Unix(reset, 0)
	}
	if value := response.Header.Get("Retry-After"); value != "" {
		if seconds, ok := parseGitHubHeaderInt64(value); ok {
			snapshot.retryAfter = max(time.Second, time.Duration(seconds)*time.Second)
			snapshot.retryAfterKnown = true
		} else if retryAt, err := http.ParseTime(value); err == nil {
			snapshot.retryAfter = max(time.Second, retryAt.Sub(now))
			snapshot.retryAfterKnown = true
		}
	}
	var payload struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(raw, &payload) == nil {
		snapshot.message = sanitizeGitHubDiagnostic(payload.Message)
	}
	return snapshot
}

func parseGitHubHeaderInt(value string) (int, bool) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	return parsed, err == nil
}

func parseGitHubHeaderInt64(value string) (int64, bool) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return parsed, err == nil
}

func sanitizeGitHubDiagnostic(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) > githubErrorMessageLimit {
		value = string(runes[:githubErrorMessageLimit]) + "…"
	}
	return value
}

func classifyGitHubResponse(snapshot githubRateLimitSnapshot, rateAttempt, transientAttempt int, now time.Time) (class string, retry bool, delay time.Duration, maxAttempts int) {
	if snapshot.status == http.StatusForbidden || snapshot.status == http.StatusTooManyRequests {
		switch {
		case snapshot.retryAfterKnown:
			return "retry-after", true, snapshot.retryAfter, githubRateLimitAttempts
		case snapshot.countersKnown && snapshot.remaining == 0:
			if snapshot.reset.IsZero() {
				return "primary-rate-limit", true, time.Minute, githubRateLimitAttempts
			}
			delay = snapshot.reset.Add(githubResetMargin).Sub(now)
			return "primary-rate-limit", true, max(time.Second, delay), githubRateLimitAttempts
		case snapshot.status == http.StatusTooManyRequests || isSecondaryRateLimitMessage(snapshot.message):
			return "secondary-rate-limit", true, secondaryRetryDelay(rateAttempt + 1), githubRateLimitAttempts
		default:
			return "forbidden", false, 0, 1
		}
	}
	if retryableGitHubStatus(snapshot.status) {
		return "server-error", true, transientRetryDelay(transientAttempt + 1), githubTransientAttempts
	}
	return "http-error", false, 0, 1
}

func isSecondaryRateLimitMessage(message string) bool {
	message = strings.ToLower(message)
	return strings.Contains(message, "secondary rate limit") || strings.Contains(message, "abuse detection")
}

func retryableGitHubStatus(status int) bool {
	switch status {
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func retryableTransportError(err error) bool {
	var networkError net.Error
	return errors.As(err, &networkError) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
}

func secondaryRetryDelay(attempt int) time.Duration {
	return time.Minute * time.Duration(1<<max(0, attempt-1))
}

func transientRetryDelay(attempt int) time.Duration {
	return time.Second * time.Duration(1<<max(0, attempt-1))
}

func (snapshot githubRateLimitSnapshot) providerProgress(reason string) ProviderProgress {
	return ProviderProgress{
		Reason: reason, Status: snapshot.status, RequestID: snapshot.requestID,
		Message: snapshot.message, Reset: snapshot.reset,
		Remaining: snapshot.remaining, Limit: snapshot.limit, CountersKnown: snapshot.countersKnown,
	}
}

func githubResponseError(snapshot githubRateLimitSnapshot, class string) error {
	diagnostic := fmt.Sprintf("GitHub HTTP status %d class=%s resource=%s", snapshot.status, class, snapshot.resource)
	if snapshot.countersKnown {
		diagnostic += fmt.Sprintf(" remaining=%d/%d", snapshot.remaining, snapshot.limit)
	}
	if !snapshot.reset.IsZero() {
		diagnostic += " reset=" + snapshot.reset.UTC().Format(time.RFC3339)
	}
	if snapshot.requestID != "" {
		diagnostic += " request-id=" + snapshot.requestID
	}
	if snapshot.message != "" {
		diagnostic += fmt.Sprintf(" message=%q", snapshot.message)
	}
	return errors.New(diagnostic)
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
	return hits, githubCodeSearchHasNext(page, len(response.Items)), n, err
}

func githubCodeSearchHasNext(page, items int) bool {
	return items == 100 && page < githubCodeSearchMaxPage
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
