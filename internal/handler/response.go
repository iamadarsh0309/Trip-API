package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iamadarsh0309/Trip-API/internal/httperr"
)

func writeError(c *gin.Context, err error) {
	if appErr, ok := httperr.AsAppError(err); ok {
		c.JSON(appErr.HTTPStatus, httperr.ToEnvelope(appErr))
		return
	}
	c.JSON(http.StatusInternalServerError, httperr.ToEnvelope(httperr.Internal("unexpected error")))
}
