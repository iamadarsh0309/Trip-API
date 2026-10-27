package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/iamadarsh0309/Trip-API/internal/middleware"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func NewRouter(h *TripHandler) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())
	r.Use(middleware.RequestID())
	r.Use(middleware.Timeout(15 * time.Second))

	r.GET("/health", h.Health)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	trips := r.Group("/trips")
	{
		trips.POST("", h.CreateTrip)
		trips.GET("", h.ListTrips)
		trips.GET("/:id", h.GetTrip)
		trips.PATCH("/:id/status", h.UpdateTripStatus)
		trips.DELETE("/:id", h.DeleteTrip)
	}

	return r
}
