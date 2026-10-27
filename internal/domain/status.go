package domain

import "fmt"

type Status string

const (
	StatusCreated   Status = "CREATED"
	StatusAssigned  Status = "ASSIGNED"
	StatusInTransit Status = "IN_TRANSIT"
	StatusCompleted Status = "COMPLETED"
	StatusCancelled Status = "CANCELLED"
)

type InvalidTransitionError struct {
	From Status
	To   Status
}

func (e *InvalidTransitionError) Error() string {
	return fmt.Sprintf("invalid status transition from %s to %s", e.From, e.To)
}

var allowedTransitions = map[Status]map[Status]struct{}{
	StatusCreated: {
		StatusAssigned:  {},
		StatusCancelled: {},
	},
	StatusAssigned: {
		StatusInTransit: {},
		StatusCancelled: {},
	},
	StatusInTransit: {
		StatusCompleted: {},
		StatusCancelled: {},
	},
	StatusCompleted: {},
	StatusCancelled: {},
}

func ParseStatus(s string) (Status, error) {
	status := Status(s)
	if !status.IsValid() {
		return "", fmt.Errorf("invalid status: %s", s)
	}
	return status, nil
}

func (s Status) IsValid() bool {
	switch s {
	case StatusCreated, StatusAssigned, StatusInTransit, StatusCompleted, StatusCancelled:
		return true
	default:
		return false
	}
}

func (s Status) IsTerminal() bool {
	return s == StatusCompleted || s == StatusCancelled
}

// CanTransitionTo reports whether from -> to is allowed.
// Same-status is treated as an allowed no-op by callers.
func CanTransitionTo(from, to Status) bool {
	if from == to {
		return true
	}
	allowed, ok := allowedTransitions[from]
	if !ok {
		return false
	}
	_, ok = allowed[to]
	return ok
}

func ValidateTransition(from, to Status) error {
	if !to.IsValid() {
		return &InvalidTransitionError{From: from, To: to}
	}
	if from == to {
		return nil
	}
	if !CanTransitionTo(from, to) {
		return &InvalidTransitionError{From: from, To: to}
	}
	return nil
}
