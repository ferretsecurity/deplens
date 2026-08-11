package main

import (
	"context"
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
