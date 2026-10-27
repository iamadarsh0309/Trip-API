package domain

import "testing"

func TestValidateTransition_HappyPath(t *testing.T) {
	path := []Status{StatusCreated, StatusAssigned, StatusInTransit, StatusCompleted}
	for i := 0; i < len(path)-1; i++ {
		if err := ValidateTransition(path[i], path[i+1]); err != nil {
			t.Fatalf("expected %s -> %s to be valid, got %v", path[i], path[i+1], err)
		}
	}
}

func TestValidateTransition_CancelAllowed(t *testing.T) {
	for _, from := range []Status{StatusCreated, StatusAssigned, StatusInTransit} {
		if err := ValidateTransition(from, StatusCancelled); err != nil {
			t.Fatalf("expected cancel from %s to be valid: %v", from, err)
		}
	}
}

func TestValidateTransition_NoopSameStatus(t *testing.T) {
	if err := ValidateTransition(StatusAssigned, StatusAssigned); err != nil {
		t.Fatalf("same-status should be no-op: %v", err)
	}
}

func TestValidateTransition_RejectsBackwardsAndJumps(t *testing.T) {
	cases := []struct {
		from Status
		to   Status
	}{
		{StatusAssigned, StatusCreated},
		{StatusInTransit, StatusAssigned},
		{StatusCreated, StatusInTransit},
		{StatusCreated, StatusCompleted},
		{StatusCompleted, StatusCancelled},
		{StatusCompleted, StatusAssigned},
		{StatusCancelled, StatusCreated},
	}

	for _, tc := range cases {
		err := ValidateTransition(tc.from, tc.to)
		if err == nil {
			t.Fatalf("expected %s -> %s to be invalid", tc.from, tc.to)
		}
		if _, ok := err.(*InvalidTransitionError); !ok {
			t.Fatalf("expected InvalidTransitionError, got %T", err)
		}
	}
}

func TestParseStatus(t *testing.T) {
	st, err := ParseStatus("IN_TRANSIT")
	if err != nil || st != StatusInTransit {
		t.Fatalf("unexpected parse result: %v %v", st, err)
	}
	if _, err := ParseStatus("FLYING"); err == nil {
		t.Fatal("expected invalid status error")
	}
}

func TestCanTransitionTo(t *testing.T) {
	if !CanTransitionTo(StatusCreated, StatusAssigned) {
		t.Fatal("CREATED -> ASSIGNED should be allowed")
	}
	if CanTransitionTo(StatusCompleted, StatusAssigned) {
		t.Fatal("COMPLETED -> ASSIGNED should be disallowed")
	}
}
