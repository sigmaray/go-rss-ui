package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go-rss-ui/validation"
	"gorm.io/gorm"
)

func parseID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid id"})
		return 0, false
	}
	return uint(id), true
}

func validationError(c *gin.Context, err error) {
	c.JSON(http.StatusBadRequest, ErrorResponse{Error: validation.FormatValidationErrors(err)})
}

func notFound(c *gin.Context, resource string) {
	c.JSON(http.StatusNotFound, ErrorResponse{Error: resource + " not found"})
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "duplicate key") ||
		strings.Contains(errStr, "unique constraint") ||
		strings.Contains(errStr, "23505") ||
		strings.Contains(errStr, "unique constraint failed")
}

func isNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
