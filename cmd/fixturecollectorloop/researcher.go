package main

import (
	"context"
	"errors"
)

// Researcher performs one detector research attempt. It returns the result to
// the collector wrapper, which remains responsible for recovery, validation,
// checkpointing, and Git operations.
type Researcher interface {
	Research(context.Context, Iteration) (ResearchResult, error)
}

type researcherPreflighter interface {
	Preflight() error
}

// ResearchResult is the in-memory output of one research attempt.
type ResearchResult struct {
	Outcome                 Outcome
	Selection               Selection
	Accepted                []AcceptedCandidate
	Decision                *DecisionState
	NoDistinctDecisionState bool
	NoDistinctResearchState bool
}

// Acquisition obtains the bounded research input for an iteration. Its
// production implementation will own remote discovery and retrieval.
type Acquisition interface {
	Acquire(context.Context, Iteration) (ResearchInput, error)
}

// Selector compares acquired candidates and returns a research result. Its
// production implementation will invoke the isolated Codex selector.
type Selector interface {
	Select(context.Context, Iteration, []byte) (ResearchResult, error)
}

type selectorConfigurationFingerprinter interface {
	configurationFingerprint() string
}

func selectorConfigurationFingerprint(iteration Iteration, selector Selector) string {
	if iteration.SelectorConfigurationFingerprint != "" {
		return iteration.SelectorConfigurationFingerprint
	}
	if fingerprint, ok := selector.(selectorConfigurationFingerprinter); ok {
		if configuration := fingerprint.configurationFingerprint(); configuration != "" {
			return configuration
		}
	}
	return hash("fixture-collector-selector-configuration-v1")
}

// ResearchInput is the bounded, in-memory handoff from acquisition to
// selection. The packet is intentionally opaque to the command wrapper.
type ResearchInput struct {
	SelectionPacket []byte
	Candidates      []SourceCandidate
	Outcome         Outcome
	SearchExhausted bool
}

type unconfiguredAcquisition struct{}

func (unconfiguredAcquisition) Acquire(context.Context, Iteration) (ResearchInput, error) {
	return ResearchInput{}, errors.New("no Go-owned acquisition adapter is configured")
}

type unconfiguredSelector struct{}

func (unconfiguredSelector) Select(context.Context, Iteration, []byte) (ResearchResult, error) {
	return ResearchResult{}, errors.New("no isolated selector adapter is configured")
}

type composedResearcher struct {
	acquisition Acquisition
	selector    Selector
}

func newComposedResearcher(acquisition Acquisition, selector Selector) Researcher {
	return composedResearcher{acquisition: acquisition, selector: selector}
}

func (r composedResearcher) Research(ctx context.Context, iteration Iteration) (ResearchResult, error) {
	input, err := r.acquisition.Acquire(ctx, iteration)
	if err != nil {
		return ResearchResult{}, err
	}
	candidates := input.Candidates
	if len(candidates) == 0 {
		input.Outcome.Result = "unsuccessful"
		return ResearchResult{Outcome: input.Outcome, NoDistinctResearchState: input.SearchExhausted}, nil
	}
	packet, err := buildSelectionPacket(SelectionPacketOptions{
		Candidates: candidates, AcceptedReferences: iteration.AcceptedReferences,
		QueryPlan: iteration.QueryPlan, PacketTokens: iteration.PacketTokens,
		PresentedIDs: iteration.PresentedCandidateIDs,
	})
	if err != nil {
		return ResearchResult{}, err
	}
	input.SelectionPacket = packet.Bytes
	candidates = packet.Candidates
	input.Outcome.Omitted = append(input.Outcome.Omitted, packet.OmittedIDs...)
	if len(candidates) < selectionCount {
		input.Outcome.Result = "unsuccessful"
		return ResearchResult{Outcome: input.Outcome, NoDistinctResearchState: input.SearchExhausted}, nil
	}
	configuration := selectorConfigurationFingerprint(iteration, r.selector)
	decision := DecisionState{PacketFingerprint: packet.PacketFingerprint, AcceptedCorpusFingerprint: packet.AcceptedFingerprint, SelectorConfiguration: configuration}
	if containsDecisionState(iteration.PriorDecisionStates, decision) {
		input.Outcome.Result = "unsuccessful"
		return ResearchResult{Outcome: input.Outcome, NoDistinctDecisionState: true, NoDistinctResearchState: input.SearchExhausted}, nil
	}
	if iteration.ReportProgress != nil {
		iteration.ReportProgress(ResearchProgress{Stage: progressSelection, Candidates: len(candidates)})
	}
	result, err := r.selector.Select(ctx, iteration, input.SelectionPacket)
	if err != nil {
		return result, err
	}
	if iteration.ReportProgress != nil {
		iteration.ReportProgress(ResearchProgress{Stage: progressSelection, Final: true, Selected: len(result.Selection.Selected)})
	}
	if err := validateSelection(result.Selection, candidateIDSet(candidates)); err != nil {
		return ResearchResult{}, err
	}
	result.Decision = &decision
	result.NoDistinctResearchState = input.SearchExhausted
	if len(input.Outcome.Queries) != 0 || len(input.Outcome.Candidates) != 0 || len(input.Outcome.FilteredSearchHits) != 0 || len(input.Outcome.Rejections) != 0 || len(input.Outcome.Omitted) != 0 || len(input.Outcome.SearchCursors) != 0 || len(input.Outcome.SearchHitIDs) != 0 {
		result.Outcome = input.Outcome
	}
	accepted, rejected, err := materializeSelectedCandidates(iteration.CorpusDir, iteration.DetectorID, candidates, result.Selection)
	if err != nil {
		return ResearchResult{}, err
	}
	result.Accepted = accepted
	result.Outcome.Rejections = append(result.Outcome.Rejections, rejected...)
	result.Outcome.Result = "unsuccessful"
	if len(accepted) > 0 {
		result.Outcome.Result = "accepted"
	}
	for _, accepted := range accepted {
		result.Outcome.Added = append(result.Outcome.Added, accepted.Directory+"/"+accepted.Candidate.OriginalPath, accepted.Directory+"/provenance.yaml")
	}
	return result, nil
}

func candidateIDSet(candidates []SourceCandidate) map[string]struct{} {
	ids := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		ids[candidate.ID] = struct{}{}
	}
	return ids
}

func containsDecisionState(states []DecisionState, wanted DecisionState) bool {
	for _, state := range states {
		if state == wanted {
			return true
		}
	}
	return false
}
