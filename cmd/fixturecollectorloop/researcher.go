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

// ResearchResult is the in-memory output of one research attempt.
type ResearchResult struct {
	Outcome Outcome
}

// Acquisition obtains the bounded research input for an iteration. Its
// production implementation will own remote discovery and retrieval.
type Acquisition interface {
	Acquire(context.Context, Iteration) (ResearchInput, error)
}

// Selector compares acquired candidates and returns a research result. Its
// production implementation will invoke the isolated Codex selector.
type Selector interface {
	Select(context.Context, Iteration, ResearchInput) (ResearchResult, error)
}

// ResearchInput is the bounded, in-memory handoff from acquisition to
// selection. The packet is intentionally opaque to the command wrapper.
type ResearchInput struct {
	SelectionPacket []byte
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
	return r.selector.Select(ctx, iteration, input)
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
	if configurable, ok := r.agent.(interface{ SetRetainLogs(bool) }); ok {
		configurable.SetRetainLogs(retain)
	}
}

func (r legacyAgentResearcher) Preflight() error {
	if preflight, ok := r.agent.(interface{ Preflight() error }); ok {
		return preflight.Preflight()
	}
	return nil
}

func (r legacyAgentResearcher) FinalizeResearch(success bool) {
	if finalizer, ok := r.agent.(interface{ FinalizeIteration(bool) }); ok {
		finalizer.FinalizeIteration(success)
	}
}

type unavailableResearcher struct{}

func (unavailableResearcher) Research(context.Context, Iteration) (ResearchResult, error) {
	return ResearchResult{}, errors.New("no Researcher is configured; inject one through the command seam")
}

// unavailableAgent preserves the old test-only command fixture while command
// callers move to the Researcher boundary.
type unavailableAgent struct{}

func (unavailableAgent) Run(context.Context, Iteration) (Outcome, error) {
	return Outcome{}, errors.New("no Codex agent is configured; inject an agent through the command seam")
}

func (a unavailableAgent) Research(ctx context.Context, iteration Iteration) (ResearchResult, error) {
	return newLegacyAgentResearcher(a).Research(ctx, iteration)
}
