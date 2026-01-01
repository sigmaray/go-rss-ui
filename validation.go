package main

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"github.com/go-playground/validator/v10"
	"github.com/microcosm-cc/bluemonday"
)

var (
	validate      *validator.Validate
	htmlSanitizer *bluemonday.Policy
)

func init() {
	validate = validator.New()

	// Register custom validators
	validate.RegisterValidation("username", validateUsername)
	// validate.RegisterValidation("password_strength", validatePasswordStrength)
	validate.RegisterValidation("http_url", validateHTTPURL)

	// Initialize HTML sanitizer with UGC (User Generated Content) policy
	// This policy allows safe HTML tags while removing dangerous ones
	htmlSanitizer = bluemonday.UGCPolicy()
}

// UserInput represents user input for creation/editing
type UserInput struct {
	Username string `validate:"required,username,min=3,max=50" json:"username"`
	// Password string `validate:"required,password_strength,min=8,max=128" json:"password"`
	Password string `validate:"required,min=8,max=128" json:"password"`
}

// UserInputUpdate represents user input for editing (password optional)
type UserInputUpdate struct {
	Username string `validate:"required,username,min=3,max=50" json:"username"`
	// Password string `validate:"omitempty,password_strength,min=8,max=128" json:"password"`
	Password string `validate:"omitempty,min=8,max=128" json:"password"`
}

// FeedInput represents feed input for creation
type FeedInput struct {
	URL string `validate:"required,http_url" json:"url"`
}

// validateUsername is a custom validator for username
// Rules: alphanumeric, underscore, hyphen; must start with letter or number
func validateUsername(fl validator.FieldLevel) bool {
	username := fl.Field().String()
	if username == "" {
		return false
	}

	// Check if starts with letter or number
	firstChar := rune(username[0])
	if !unicode.IsLetter(firstChar) && !unicode.IsNumber(firstChar) {
		return false
	}

	// Check if contains only allowed characters
	usernameRegex := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	return usernameRegex.MatchString(username)
}

// // validatePasswordStrength is a custom validator for password
// // Rules: must contain at least one letter and one number
// func validatePasswordStrength(fl validator.FieldLevel) bool {
// 	password := fl.Field().String()
// 	if password == "" {
// 		return false
// 	}

// 	hasLetter := false
// 	hasNumber := false

// 	for _, char := range password {
// 		if unicode.IsLetter(char) {
// 			hasLetter = true
// 		}
// 		if unicode.IsNumber(char) {
// 			hasNumber = true
// 		}
// 	}

// 	return hasLetter && hasNumber
// }

// validateHTTPURL is a custom validator for URL
// Rules: must be valid URL with http or https protocol
func validateHTTPURL(fl validator.FieldLevel) bool {
	urlString := strings.TrimSpace(fl.Field().String())
	if urlString == "" {
		return false
	}

	parsedURL, err := url.Parse(urlString)
	if err != nil {
		return false
	}

	// Check if scheme is http or https
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return false
	}

	// Check if host is present
	if parsedURL.Host == "" {
		return false
	}

	return true
}

// ValidateStruct validates a struct using validator/v10
func ValidateStruct(s interface{}) error {
	return validate.Struct(s)
}

// FormatValidationErrors formats validator errors into a user-friendly message
func FormatValidationErrors(err error) string {
	if err == nil {
		return ""
	}

	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		return err.Error()
	}

	var messages []string
	for _, fieldError := range validationErrors {
		field := fieldError.Field()
		tag := fieldError.Tag()

		var message string
		switch tag {
		case "required":
			message = fmt.Sprintf("%s is required", field)
		case "min":
			message = fmt.Sprintf("%s must be at least %s characters long", field, fieldError.Param())
		case "max":
			message = fmt.Sprintf("%s must be at most %s characters long", field, fieldError.Param())
		case "username":
			message = fmt.Sprintf("%s can only contain letters, numbers, underscores, and hyphens, and must start with a letter or number", field)
		// case "password_strength":
		// 	message = fmt.Sprintf("%s must contain at least one letter and one number", field)
		case "http_url":
			message = fmt.Sprintf("%s must be a valid URL with http or https protocol", field)
		case "omitempty":
			// Skip if field is empty (for optional fields)
			continue
		default:
			message = fmt.Sprintf("%s is invalid", field)
		}
		messages = append(messages, message)
	}

	return strings.Join(messages, "; ")
}

// SanitizeHTML sanitizes HTML content using bluemonday library
// Uses UGC (User Generated Content) policy which allows safe HTML tags
// while removing dangerous ones like script, iframe, object, embed, etc.
func SanitizeHTML(html string) string {
	if html == "" {
		return ""
	}

	// Use bluemonday UGCPolicy to sanitize HTML
	// This policy:
	// - Removes script, style, iframe, object, embed tags
	// - Removes event handlers (onclick, onerror, etc.)
	// - Removes javascript: protocol in href/src
	// - Allows safe HTML tags like p, div, a, img, etc.
	return htmlSanitizer.Sanitize(html)
}
