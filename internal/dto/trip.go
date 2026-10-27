package dto

import (
	"time"

	"github.com/iamadarsh0309/Trip-API/internal/domain"
)

type CreateTripRequest struct {
	TripNumber  string `json:"trip_number" binding:"required,min=1,max=64" example:"TRIP-1001"`
	Source      string `json:"source" binding:"required,min=1,max=255" example:"Bangalore"`
	Destination string `json:"destination" binding:"required,min=1,max=255" example:"Hyderabad"`
	DriverName  string `json:"driver_name" binding:"required,min=1,max=255" example:"Rahul Sharma"`
}

type UpdateStatusRequest struct {
	Status string `json:"status" binding:"required" example:"ASSIGNED"`
}

type TripResponse struct {
	ID          string    `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	TripNumber  string    `json:"trip_number" example:"TRIP-1001"`
	Source      string    `json:"source" example:"Bangalore"`
	Destination string    `json:"destination" example:"Hyderabad"`
	DriverName  string    `json:"driver_name" example:"Rahul Sharma"`
	Status      string    `json:"status" example:"CREATED"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ListTripsResponse struct {
	Items    []TripResponse `json:"items"`
	Page     int            `json:"page" example:"1"`
	PageSize int            `json:"page_size" example:"20"`
	Total    int64          `json:"total" example:"42"`
}

type ListTripsQuery struct {
	Status   string `form:"status"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

func FromDomain(t *domain.Trip) TripResponse {
	return TripResponse{
		ID:          t.ID.String(),
		TripNumber:  t.TripNumber,
		Source:      t.Source,
		Destination: t.Destination,
		DriverName:  t.DriverName,
		Status:      string(t.Status),
		CreatedAt:   t.CreatedAt.UTC(),
		UpdatedAt:   t.UpdatedAt.UTC(),
	}
}

func FromDomainList(trips []domain.Trip) []TripResponse {
	items := make([]TripResponse, 0, len(trips))
	for i := range trips {
		items = append(items, FromDomain(&trips[i]))
	}
	return items
}
