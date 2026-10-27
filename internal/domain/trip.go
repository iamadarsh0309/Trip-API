package domain

import (
	"time"

	"github.com/google/uuid"
)

type Trip struct {
	ID          uuid.UUID
	TripNumber  string
	Source      string
	Destination string
	DriverName  string
	Status      Status
	Version     int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
