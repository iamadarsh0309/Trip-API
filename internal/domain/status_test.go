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
	for _, st := range []Status{StatusCreated, StatusAssigned, StatusInTransit, StatusCompleted, StatusCancelled} {
		if err := ValidateTransition(st, st); err != nil {
			t.Fatalf("same-status %s should be no-op: %v", st, err)
		}
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
		{StatusAssigned, StatusCompleted},
		{StatusCompleted, StatusCancelled},
		{StatusCompleted, StatusAssigned},
		{StatusCancelled, StatusCreated},
		{StatusCancelled, StatusAssigned},
		{StatusInTransit, StatusCreated},
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
	if _, err := ParseStatus(""); err == nil {
		t.Fatal("expected empty status to fail")
	}
}

func TestCanTransitionTo(t *testing.T) {
	if !CanTransitionTo(StatusCreated, StatusAssigned) {
		t.Fatal("CREATED -> ASSIGNED should be allowed")
	}
	if !CanTransitionTo(StatusCreated, StatusCreated) {
		t.Fatal("same status should be allowed")
	}
	if CanTransitionTo(StatusCompleted, StatusAssigned) {
		t.Fatal("COMPLETED -> ASSIGNED should be disallowed")
	}
}

func TestIsTerminal(t *testing.T) {
	if !StatusCompleted.IsTerminal() || !StatusCancelled.IsTerminal() {
		t.Fatal("completed/cancelled should be terminal")
	}
	if StatusCreated.IsTerminal() || StatusAssigned.IsTerminal() || StatusInTransit.IsTerminal() {
		t.Fatal("non-terminal statuses marked terminal")
	}
}

func TestIsValid(t *testing.T) {
	for _, st := range []Status{StatusCreated, StatusAssigned, StatusInTransit, StatusCompleted, StatusCancelled} {
		if !st.IsValid() {
			t.Fatalf("%s should be valid", st)
		}
	}
	if Status("NOPE").IsValid() {
		t.Fatal("invalid status reported valid")
	}
}

func TestInvalidTransitionErrorMessage(t *testing.T) {
	err := &InvalidTransitionError{From: StatusCreated, To: StatusCompleted}
	want := "invalid status transition from CREATED to COMPLETED"
	if err.Error() != want {
		t.Fatalf("got %q want %q", err.Error(), want)
	}
}
