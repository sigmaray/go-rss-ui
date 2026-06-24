package validation_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go-rss-ui/validation"
)

func TestUserInputCreate(t *testing.T) {
	tests := []struct {
		name        string
		input       validation.UserInputCreate
		expectError bool
		errorSubstr string
	}{
		{
			name: "valid input",
			input: validation.UserInputCreate{
				Username:        "newuser",
				Password:        "password123",
				PasswordConfirm: "password123",
			},
		},
		{
			name: "passwords do not match",
			input: validation.UserInputCreate{
				Username:        "newuser",
				Password:        "password123",
				PasswordConfirm: "different",
			},
			expectError: true,
			errorSubstr: "PasswordConfirm must match Password",
		},
		{
			name: "invalid username",
			input: validation.UserInputCreate{
				Username:        "_invalid",
				Password:        "password123",
				PasswordConfirm: "password123",
			},
			expectError: true,
			errorSubstr: "Username",
		},
		{
			name: "password too short",
			input: validation.UserInputCreate{
				Username:        "validuser",
				Password:        "short",
				PasswordConfirm: "short",
			},
			expectError: true,
			errorSubstr: "Password must be at least 8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validation.ValidateStruct(tt.input)

			if tt.expectError {
				assert.Error(t, err)
				assert.True(t, strings.Contains(validation.FormatValidationErrors(err), tt.errorSubstr))
				return
			}

			assert.NoError(t, err)
		})
	}
}
