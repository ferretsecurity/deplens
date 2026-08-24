package main

import (
	"reflect"
	"testing"

	"github.com/ferretsecurity/deplens/internal/analyzerloop"
)

func TestRunSelectionDefaultsToFirstUnfinishedItem(t *testing.T) {
	ledger := analyzerloop.Ledger{WorkItems: []analyzerloop.WorkItem{
		{Number: 1, State: analyzerloop.StateCompleted},
		{Number: 2, State: analyzerloop.StateInProgress},
	}}
	got, err := runSelection("", false, ledger)
	if err != nil {
		t.Fatal(err)
	}
	if want := []int{2}; !reflect.DeepEqual(got, want) {
		t.Fatalf("selection = %v, want %v", got, want)
	}
}

func TestRunSelectionOnceSkipsCompletedSelectedItems(t *testing.T) {
	ledger := analyzerloop.Ledger{WorkItems: []analyzerloop.WorkItem{
		{Number: 1, State: analyzerloop.StateCompleted},
		{Number: 2, State: analyzerloop.StatePending},
	}}
	got, err := runSelection("1,2", true, ledger)
	if err != nil {
		t.Fatal(err)
	}
	if want := []int{2}; !reflect.DeepEqual(got, want) {
		t.Fatalf("selection = %v, want %v", got, want)
	}
}
