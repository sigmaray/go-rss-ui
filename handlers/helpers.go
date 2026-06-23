package handlers

import (
	"log"
	"reflect"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// getTemplateData collects all template data from context (auth, flash messages, CYPRESS mode)
func getTemplateData(c *gin.Context, data gin.H) gin.H {
	if data == nil {
		data = gin.H{}
	}

	// Add authentication info
	if isAuth, exists := c.Get("isAuthenticated"); exists {
		data["isAuthenticated"] = isAuth
	}
	if username, exists := c.Get("username"); exists {
		data["username"] = username
	}

	// Add flash messages
	if success, exists := c.Get("success"); exists {
		data["success"] = success
	}
	if error, exists := c.Get("error"); exists {
		data["error"] = error
	}

	// Add CYPRESS mode info
	if isCypressMode, exists := c.Get("isCypressMode"); exists {
		data["isCypressMode"] = isCypressMode
	}

	// Add current path for active menu highlighting
	data["currentPath"] = c.Request.URL.Path

	return data
}

// addPaginationData adds pagination data (page, pages, prevPage, nextPage) to the data map
// It extracts Page and TotalPages fields from the page object using reflection
// baseURL is the base URL for pagination links (e.g., "/admin/users")
// entityName is the name of the entity for display (e.g., "users")
func addPaginationData(data gin.H, page interface{}, baseURL, entityName string) gin.H {
	if data == nil {
		data = gin.H{}
	}

	// Use reflection to access Page and TotalPages fields
	v := reflect.ValueOf(page)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		// If not a struct, just add the page object
		data["page"] = page
		return data
	}

	// Get Page field
	pageField := v.FieldByName("Page")
	totalPagesField := v.FieldByName("TotalPages")

	if !pageField.IsValid() || !totalPagesField.IsValid() {
		// If fields don't exist, just add the page object
		data["page"] = page
		return data
	}

	// Convert to int64
	var pageNum, totalPages int64
	if pageField.Kind() == reflect.Int64 {
		pageNum = pageField.Int()
	} else if pageField.Kind() == reflect.Int {
		pageNum = int64(pageField.Int())
	}

	if totalPagesField.Kind() == reflect.Int64 {
		totalPages = totalPagesField.Int()
	} else if totalPagesField.Kind() == reflect.Int {
		totalPages = int64(totalPagesField.Int())
	}

	// Ensure page is at least 1
	if pageNum < 1 {
		pageNum = 1
	}

	prevPage := pageNum - 1
	if prevPage < 1 {
		prevPage = 0 // 0 means no previous page
	}

	nextPage := pageNum + 1
	if nextPage > totalPages {
		nextPage = 0 // 0 means no next page
	}

	data["page"] = page
	data["pages"] = generatePageNumbers(pageNum, totalPages)
	data["prevPage"] = prevPage
	data["nextPage"] = nextPage
	data["paginationBaseURL"] = baseURL
	data["paginationEntityName"] = entityName

	return data
}

// Helper functions for flash messages using simple strings instead of maps
func addFlashSuccess(session sessions.Session, message string) {
	session.AddFlash("success:" + message)
}

func addFlashError(session sessions.Session, message string) {
	session.AddFlash("error:" + message)
}

func getFlashMessages(session sessions.Session) (successMsg, errorMsg string) {
	flashes := session.Flashes()
	for _, flash := range flashes {
		if flashStr, ok := flash.(string); ok {
			if len(flashStr) > 8 && flashStr[:8] == "success:" {
				successMsg = flashStr[8:]
			} else if len(flashStr) > 6 && flashStr[:6] == "error:" {
				errorMsg = flashStr[6:]
			}
		}
	}
	return successMsg, errorMsg
}

// isUniqueConstraintError checks if the error is a unique constraint violation
func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	// Check for PostgreSQL unique constraint violation
	// PostgreSQL error code 23505 is "unique_violation"
	return strings.Contains(errStr, "duplicate key") ||
		strings.Contains(errStr, "unique constraint") ||
		strings.Contains(errStr, "23505") ||
		strings.Contains(errStr, "unique constraint failed")
}

// generatePageNumbers generates a slice of page numbers for pagination
// Returns a slice where -1 represents ellipsis
func generatePageNumbers(currentPage, totalPages int64) []interface{} {
	var pages []interface{}
	if totalPages <= 7 {
		// Show all pages if 7 or fewer
		for i := int64(1); i <= totalPages; i++ {
			pages = append(pages, i)
		}
	} else {
		// Show first page
		pages = append(pages, int64(1))

		// Calculate start and end
		start := currentPage - 2
		if start < 2 {
			start = 2
		}
		end := currentPage + 2
		if end > totalPages-1 {
			end = totalPages - 1
		}

		// Add ellipsis if needed
		if start > 2 {
			pages = append(pages, int64(-1)) // -1 means ellipsis
		}

		// Add pages around current
		for i := start; i <= end; i++ {
			pages = append(pages, i)
		}

		// Add ellipsis if needed
		if end < totalPages-1 {
			pages = append(pages, int64(-1)) // -1 means ellipsis
		}

		// Show last page
		pages = append(pages, totalPages)
	}
	return pages
}

func saveSession(session sessions.Session) {
	if err := session.Save(); err != nil {
		log.Printf("Error saving session: %v", err)
	}
}
