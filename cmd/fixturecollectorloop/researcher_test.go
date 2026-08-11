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
