package main

import (
	"context"
	"testing"
)

func TestComposedResearcherReplacesAcquisitionAndSelectorIndependently(t *testing.T) {
	acquisition := fakeAcquisition{input: ResearchInput{SelectionPacket: []byte(`{"candidates":["candidate-1"]}`)}}
	selector := fakeSelector{result: ResearchResult{Outcome: Outcome{Result: "unsuccessful"}}}
	researcher := newComposedResearcher(&acquisition, &selector)
	iteration := Iteration{DetectorID: "example-detector", Iteration: 1}

	result, err := researcher.Research(context.Background(), iteration)
	if err != nil {
		t.Fatal(err)
	}
	if !acquisition.called || !selector.called {
		t.Fatalf("acquisition called = %t, selector called = %t", acquisition.called, selector.called)
	}
	if selector.input.SelectionPacket == nil || result.Outcome.Result != "unsuccessful" {
		t.Fatalf("selector input = %+v, result = %+v", selector.input, result)
	}
}

func TestLegacyAgentResearcherDelegatesResearchLifecycle(t *testing.T) {
	agent := &lifecycleAgent{outcome: Outcome{Result: "unsuccessful"}, retainLogs: true}
	researcher := newLegacyAgentResearcher(agent)

	if err := researcher.(researcherPreflighter).Preflight(); err != nil {
		t.Fatal(err)
	}
	researcher.(researchLogConfigurer).SetRetainLogs(false)
	result, err := researcher.Research(context.Background(), Iteration{DetectorID: "example-detector"})
	if err != nil {
		t.Fatal(err)
	}
	researcher.(researchFinalizer).FinalizeResearch(true)

	if result.Outcome.Result != agent.outcome.Result || !agent.preflightCalled || agent.retainLogs || !agent.finalized || !agent.finalizeSuccess {
		t.Fatalf("agent lifecycle = %+v", agent)
	}
}

type fakeAcquisition struct {
	called bool
	input  ResearchInput
}

func (f *fakeAcquisition) Acquire(_ context.Context, _ Iteration) (ResearchInput, error) {
	f.called = true
	return f.input, nil
}

type fakeSelector struct {
	called bool
	input  ResearchInput
	result ResearchResult
}

func (f *fakeSelector) Select(_ context.Context, _ Iteration, input ResearchInput) (ResearchResult, error) {
	f.called = true
	f.input = input
	return f.result, nil
}

type lifecycleAgent struct {
	outcome         Outcome
	preflightCalled bool
	retainLogs      bool
	finalized       bool
	finalizeSuccess bool
}

func (a *lifecycleAgent) Run(context.Context, Iteration) (Outcome, error) {
	return a.outcome, nil
}

func (a *lifecycleAgent) SetRetainLogs(retain bool) {
	a.retainLogs = retain
}

func (a *lifecycleAgent) Preflight() error {
	a.preflightCalled = true
	return nil
}

func (a *lifecycleAgent) FinalizeIteration(success bool) {
	a.finalized = true
	a.finalizeSuccess = success
}
