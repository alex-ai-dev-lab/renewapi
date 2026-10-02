package common

import "testing"

func TestMappedCandidateSharesAttemptBudgetAndPreservesChannelHistory(t *testing.T) {
	state := NewChannelFailoverState()
	if !state.Begin(1, "first", 10) {
		t.Fatal("first channel refused")
	}
	state.AttemptRecords[0].UpstreamModel = "A"
	if !state.BeginMappedCandidate(1, "first", 10, "B") {
		t.Fatal("new target refused")
	}
	state.AttemptRecords[1].UpstreamModel = "B"
	if state.AttemptCount != 2 || state.SwitchCount != 0 || !state.AttemptedChannelIDs[1] {
		t.Fatalf("invalid same-channel accounting: %+v", state)
	}
	if state.BeginMappedCandidate(1, "first", 10, "A") || state.Begin(1, "first", 10) {
		t.Fatal("duplicate target/channel accepted")
	}
	if !state.Begin(2, "second", 5) {
		t.Fatal("next channel refused")
	}
	if state.SwitchCount != 1 {
		t.Fatal("candidate counted as channel switch")
	}
	if state.BeginMappedCandidate(1, "first", 10, "C") {
		t.Fatal("returned to previously exhausted channel")
	}
	for _, target := range []string{"D", "E", "F"} {
		if !state.BeginMappedCandidate(2, "second", 5, target) {
			t.Fatal("candidate refused before global budget exhausted")
		}
		state.AttemptRecords[len(state.AttemptRecords)-1].UpstreamModel = target
	}
	if state.CanAttempt() || state.Begin(3, "third", 0) || state.BeginMappedCandidate(2, "second", 5, "G") {
		t.Fatal("global budget was reset")
	}
	if state.AttemptCount != 6 || state.SwitchCount != 1 || len(state.AttemptRecords) != 6 {
		t.Fatalf("invalid global accounting: %+v", state)
	}
}
