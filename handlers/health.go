package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Health responds to uptime and container health probes.
func Health(c *gin.Context) {
	if c.Request.Method == http.MethodHead {
		c.Status(http.StatusOK)
		return
	}
	c.String(http.StatusOK, "ok")
}
