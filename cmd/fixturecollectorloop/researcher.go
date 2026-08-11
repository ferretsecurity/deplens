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

// Optional Researcher capabilities preserve the legacy session lifecycle while
// allowing new implementations to own only research.
type researchLogConfigurer interface {
	SetRetainLogs(bool)
}

type researcherPreflighter interface {
	Preflight() error
}

type researchFinalizer interface {
	FinalizeResearch(bool)
}

type iterationFinalizer interface {
	FinalizeIteration(bool)
}

// ResearchResult is the in-memory output of one research attempt.
type ResearchResult struct {
	Outcome   Outcome
	Selection Selection
	Accepted  []AcceptedCandidate
	Decision  *DecisionState
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
	if len(candidates) != 0 {
		packet, packetErr := buildSelectionPacket(SelectionPacketOptions{
			Candidates: candidates, AcceptedReferences: iteration.AcceptedReferences,
			QueryPlan: iteration.QueryPlan, PacketTokens: iteration.PacketTokens,
			PresentedIDs: iteration.PresentedCandidateIDs,
		})
		err = packetErr
		if err != nil {
			return ResearchResult{}, err
		}
		input.SelectionPacket = packet.Bytes
		candidates = packet.Candidates
		input.Outcome.Omitted = append(input.Outcome.Omitted, packet.OmittedIDs...)
		configuration := selectorConfigurationFingerprint(iteration, r.selector)
		decision := DecisionState{PacketFingerprint: packet.PacketFingerprint, AcceptedCorpusFingerprint: packet.AcceptedFingerprint, SelectorConfiguration: configuration}
		if containsDecisionState(iteration.PriorDecisionStates, decision) {
			input.Outcome.Result = "unsuccessful"
			return ResearchResult{Outcome: input.Outcome}, nil
		}
		result, err := r.selector.Select(ctx, iteration, input.SelectionPacket)
		if err != nil {
			return result, err
		}
		if err := validateSelection(result.Selection, candidateIDSet(candidates), len(iteration.AcceptedReferences)); err != nil {
			return ResearchResult{}, err
		}
		result.Decision = &decision
		if len(input.Outcome.Queries) != 0 || len(input.Outcome.Candidates) != 0 || len(input.Outcome.Rejections) != 0 || len(input.Outcome.Omitted) != 0 {
			result.Outcome = input.Outcome
		}
		if len(candidates) == 0 {
			return result, nil
		}
		accepted, err := materializeCandidatesWithAcceptedCount(iteration.CorpusDir, iteration.DetectorID, candidates, result.Selection, len(iteration.AcceptedReferences))
		if err != nil {
			return ResearchResult{}, err
		}
		result.Accepted = accepted
		if len(accepted) > 0 {
			result.Outcome.Result = "accepted"
			for _, accepted := range accepted {
				result.Outcome.Added = append(result.Outcome.Added, accepted.Directory+"/"+accepted.Candidate.OriginalPath, accepted.Directory+"/provenance.yaml")
			}
		} else {
			result.Outcome.Result = "unsuccessful"
		}
		return result, nil
	}
	result, err := r.selector.Select(ctx, iteration, input.SelectionPacket)
	if len(input.Outcome.Queries) != 0 || len(input.Outcome.Candidates) != 0 || len(input.Outcome.Rejections) != 0 {
		result.Outcome = input.Outcome
	}
	if err != nil || len(candidates) == 0 {
		return result, err
	}
	accepted, err := materializeCandidatesWithAcceptedCount(iteration.CorpusDir, iteration.DetectorID, candidates, result.Selection, len(iteration.AcceptedReferences))
	if err != nil {
		return ResearchResult{}, err
	}
	result.Accepted = accepted
	result.Outcome.Result = "accepted"
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

// Agent is the temporary implementation boundary retained while research is
// migrated out of the existing Codex session.
type Agent interface {
	Run(context.Context, Iteration) (Outcome, error)
}

type legacyAgentResearcher struct{ agent Agent }

func newLegacyAgentResearcher(agent Agent) Researcher {
	return legacyAgentResearcher{agent: agent}
}

func (r legacyAgentResearcher) Research(ctx context.Context, iteration Iteration) (ResearchResult, error) {
	outcome, err := r.agent.Run(ctx, iteration)
	return ResearchResult{Outcome: outcome}, err
}

func (r legacyAgentResearcher) SetRetainLogs(retain bool) {
	if configurable, ok := r.agent.(researchLogConfigurer); ok {
		configurable.SetRetainLogs(retain)
	}
}

func (r legacyAgentResearcher) Preflight() error {
	if preflight, ok := r.agent.(researcherPreflighter); ok {
		return preflight.Preflight()
	}
	return nil
}

func (r legacyAgentResearcher) FinalizeResearch(success bool) {
	if finalizer, ok := r.agent.(iterationFinalizer); ok {
		finalizer.FinalizeIteration(success)
	}
}
