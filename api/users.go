package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go-rss-ui/database"
	"go-rss-ui/models"
	"go-rss-ui/services"
	"go-rss-ui/validation"
)

// ListUsers godoc
// @Summary      List users
// @Description  Returns paginated list of users
// @Tags         users
// @Produce      json
// @Param        page  query     int  false  "Page number"  default(1)
// @Success      200   {object}  PaginatedUsersResponse
// @Failure      401   {object}  ErrorResponse
// @Security     CookieAuth
// @Router       /users [get]
func ListUsers(c *gin.Context) {
	var users []models.User
	model := database.DB.Model(&models.User{}).Order("created_at DESC")
	page := database.Paginator.With(model).Request(c.Request).Response(&users)

	items := make([]UserResponse, 0, len(users))
	for _, user := range users {
		items = append(items, toUserResponse(user))
	}

	c.JSON(http.StatusOK, PaginatedUsersResponse{
		Page:       page.Page,
		TotalPages: page.TotalPages,
		Total:      page.Total,
		Items:      items,
	})
}

// GetUser godoc
// @Summary      Get user
// @Description  Returns a single user by ID
// @Tags         users
// @Produce      json
// @Param        id   path      int  true  "User ID"
// @Success      200  {object}  UserResponse
// @Failure      400  {object}  ErrorResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Security     CookieAuth
// @Router       /users/{id} [get]
func GetUser(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var user models.User
	if err := database.DB.First(&user, id).Error; err != nil {
		notFound(c, "user")
		return
	}

	c.JSON(http.StatusOK, toUserResponse(user))
}

// CreateUser godoc
// @Summary      Create user
// @Description  Create a new user
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        user  body      validation.UserInput  true  "User data"
// @Success      201   {object}  UserResponse
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Security     CookieAuth
// @Router       /users [post]
func CreateUser(c *gin.Context) {
	var input validation.UserInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	user, err := services.CreateUser(input.Username, input.Password)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, toUserResponse(user))
}

// UpdateUser godoc
// @Summary      Update user
// @Description  Update an existing user
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id    path      int                       true  "User ID"
// @Param        user  body      validation.UserInputUpdate  true  "User data"
// @Success      200   {object}  UserResponse
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Failure      404   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Security     CookieAuth
// @Router       /users/{id} [put]
func UpdateUser(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var user models.User
	if err := database.DB.First(&user, id).Error; err != nil {
		notFound(c, "user")
		return
	}

	var input validation.UserInputUpdate
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	if err := validation.ValidateStruct(input); err != nil {
		validationError(c, err)
		return
	}

	var existingUser models.User
	if err := database.DB.Where("username = ? AND id != ?", input.Username, id).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "username already exists"})
		return
	} else if !isNotFound(err) {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	user.Username = input.Username
	if input.Password != "" {
		user.Password = input.Password
	}

	if err := database.DB.Save(&user).Error; err != nil {
		if isUniqueConstraintError(err) {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "username already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, toUserResponse(user))
}

// DeleteUser godoc
// @Summary      Delete user
// @Description  Permanently delete a user
// @Tags         users
// @Produce      json
// @Param        id   path      int  true  "User ID"
// @Success      204  "No Content"
// @Failure      400  {object}  ErrorResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Security     CookieAuth
// @Router       /users/{id} [delete]
func DeleteUser(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var user models.User
	if err := database.DB.First(&user, id).Error; err != nil {
		notFound(c, "user")
		return
	}

	if err := database.DB.Unscoped().Delete(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}
