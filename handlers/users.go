package handlers

import (
	"log"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"go-rss-ui/database"
	"go-rss-ui/models"
	"go-rss-ui/validation"
)

func AdminIndex(c *gin.Context) {
	c.Redirect(http.StatusFound, "/admin/users")
}

func UsersIndex(c *gin.Context) {
	var users []models.User
	model := database.DB.Model(&models.User{}).Order("created_at DESC")
	page := database.Paginator.With(model).Request(c.Request).Response(&users)

	data := gin.H{
		"title": "User Management",
		"users": page.Items,
	}

	data = addPaginationData(data, page, "/admin/users", "users")

	// Check for error in query parameter (for backward compatibility)
	if queryError := c.Query("error"); queryError != "" {
		if _, exists := c.Get("error"); !exists {
			data["error"] = queryError
		}
	}

	data = getTemplateData(c, data)
	c.HTML(http.StatusOK, "users.html", data)
}

func ShowCreateUserForm(c *gin.Context) {
	data := getTemplateData(c, gin.H{
		"title": "Create New User",
	})
	c.HTML(http.StatusOK, "create_user.html", data)
}

func ShowEditUserForm(c *gin.Context) {
	id := c.Param("id")

	var user models.User
	if err := database.DB.First(&user, id).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/users?error=User+not+found")
		return
	}

	data := getTemplateData(c, gin.H{
		"title": "Edit User",
		"user":  user,
	})
	c.HTML(http.StatusOK, "edit_user.html", data)
}

func CreateUser(c *gin.Context) {
	// Create input struct from form data
	input := validation.UserInput{
		Username: c.PostForm("username"),
		Password: c.PostForm("password"),
	}

	// Validate input using validator/v10
	if err := validation.ValidateStruct(input); err != nil {
		data := getTemplateData(c, gin.H{
			"title": "Create New User",
			"error": validation.FormatValidationErrors(err),
		})
		c.HTML(http.StatusBadRequest, "create_user.html", data)
		return
	}

	// Check if username already exists
	var existingUser models.User
	if err := database.DB.Where("username = ?", input.Username).First(&existingUser).Error; err == nil {
		// User with this username already exists
		data := getTemplateData(c, gin.H{
			"title": "Create New User",
			"error": "Username already exists",
		})
		c.HTML(http.StatusBadRequest, "create_user.html", data)
		return
	}

	user := models.User{Username: input.Username, Password: input.Password}
	if err := database.DB.Create(&user).Error; err != nil {
		// Check if error is due to unique constraint violation
		if isUniqueConstraintError(err) {
			data := getTemplateData(c, gin.H{
				"title": "Create New User",
				"error": "Username already exists",
			})
			c.HTML(http.StatusBadRequest, "create_user.html", data)
			return
		}
		data := getTemplateData(c, gin.H{
			"title": "Create New User",
			"error": "Failed to create user: " + err.Error(),
		})
		c.HTML(http.StatusInternalServerError, "create_user.html", data)
		return
	}

	session := sessions.Default(c)
	addFlashSuccess(session, "User created successfully")
	if err := session.Save(); err != nil {
		log.Printf("Error saving session in CreateUser: %v", err)
	}
	c.Redirect(http.StatusFound, "/admin/users")
}

func EditUser(c *gin.Context) {
	id := c.Param("id")
	username := c.PostForm("username")
	password := c.PostForm("password")

	var user models.User
	if err := database.DB.First(&user, id).Error; err != nil {
		data := getTemplateData(c, gin.H{
			"title": "Edit User",
			"error": "User not found",
			"user":  user,
		})
		c.HTML(http.StatusNotFound, "edit_user.html", data)
		return
	}

	// Use current username if not provided, otherwise use new one
	usernameToValidate := username
	if usernameToValidate == "" {
		usernameToValidate = user.Username
	}

	// Create input struct for validation
	input := validation.UserInputUpdate{
		Username: usernameToValidate,
		Password: password,
	}

	// Validate input using validator/v10
	if err := validation.ValidateStruct(input); err != nil {
		data := getTemplateData(c, gin.H{
			"title": "Edit User",
			"error": validation.FormatValidationErrors(err),
			"user":  user,
		})
		c.HTML(http.StatusBadRequest, "edit_user.html", data)
		return
	}

	if username != "" {
		// Check if new username is already taken by another user
		var existingUser models.User
		if err := database.DB.Where("username = ? AND id != ?", username, id).First(&existingUser).Error; err == nil {
			data := getTemplateData(c, gin.H{
				"title": "Edit User",
				"error": "Username already exists",
				"user":  user,
			})
			c.HTML(http.StatusBadRequest, "edit_user.html", data)
			return
		}
		user.Username = username
	}
	if password != "" {
		user.Password = password
	}

	if err := database.DB.Save(&user).Error; err != nil {
		// Check if error is due to unique constraint violation
		if isUniqueConstraintError(err) {
			data := getTemplateData(c, gin.H{
				"title": "Edit User",
				"error": "Username already exists",
				"user":  user,
			})
			c.HTML(http.StatusBadRequest, "edit_user.html", data)
			return
		}
		data := getTemplateData(c, gin.H{
			"title": "Edit User",
			"error": "Failed to update user: " + err.Error(),
			"user":  user,
		})
		c.HTML(http.StatusInternalServerError, "edit_user.html", data)
		return
	}

	c.Redirect(http.StatusFound, "/admin/users")
}

func DeleteUser(c *gin.Context) {
	id := c.Param("id")
	session := sessions.Default(c)

	var user models.User
	if err := database.DB.First(&user, id).Error; err != nil {
		addFlashError(session, "User not found")
		saveSession(session)
		c.Redirect(http.StatusFound, "/admin/users")
		return
	}

	if err := database.DB.Unscoped().Delete(&user).Error; err != nil {
		addFlashError(session, "Failed to delete user: "+err.Error())
		saveSession(session)
		c.Redirect(http.StatusFound, "/admin/users")
		return
	}

	addFlashSuccess(session, "User deleted successfully")
	saveSession(session)
	c.Redirect(http.StatusFound, "/admin/users")
}
