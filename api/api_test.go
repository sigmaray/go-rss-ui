package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/morkid/paginate"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go-rss-ui/api"
	"go-rss-ui/app"
	"go-rss-ui/database"
	"go-rss-ui/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&models.User{}, &models.Feed{}, &models.Item{})
	require.NoError(t, err)
	return db
}

func setupTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	database.DB = db
	database.Paginator = paginate.New(&paginate.Config{
		DefaultSize: 50,
		PageStart:   1,
	})

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	app.Logger = &logger

	router := gin.New()
	store := cookie.NewStore([]byte("test-session-secret"))
	router.Use(sessions.Sessions("mysession", store))
	api.RegisterRoutes(router)
	return router
}

func seedAdminUser(t *testing.T) models.User {
	t.Helper()
	user := models.User{Username: "admin", Password: "password123"}
	require.NoError(t, database.DB.Create(&user).Error)
	return user
}

func apiLogin(t *testing.T, router *gin.Engine, username, password string) *http.Cookie {
	t.Helper()
	body, err := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	cookies := w.Result().Cookies()
	require.NotEmpty(t, cookies)
	return cookies[0]
}

func authenticatedRequest(t *testing.T, router *gin.Engine, method, path string, body []byte, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestAuthLoginSuccess(t *testing.T) {
	router := setupTestRouter(t)
	seedAdminUser(t)

	w := authenticatedRequest(t, router, http.MethodPost, "/api/v1/auth/login",
		[]byte(`{"username":"admin","password":"password123"}`), nil)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.UserResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "admin", resp.Username)
	assert.NotZero(t, resp.ID)
}

func TestAuthLoginInvalidCredentials(t *testing.T) {
	router := setupTestRouter(t)
	seedAdminUser(t)

	w := authenticatedRequest(t, router, http.MethodPost, "/api/v1/auth/login",
		[]byte(`{"username":"admin","password":"wrong"}`), nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var resp api.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "invalid credentials", resp.Error)
}

func TestAuthLogout(t *testing.T) {
	router := setupTestRouter(t)
	seedAdminUser(t)
	cookie := apiLogin(t, router, "admin", "password123")

	w := authenticatedRequest(t, router, http.MethodPost, "/api/v1/auth/logout", nil, cookie)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUsersUnauthorized(t *testing.T) {
	router := setupTestRouter(t)

	w := authenticatedRequest(t, router, http.MethodGet, "/api/v1/users", nil, nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestUsersCRUD(t *testing.T) {
	router := setupTestRouter(t)
	seedAdminUser(t)
	cookie := apiLogin(t, router, "admin", "password123")

	// Create user
	createBody := []byte(`{"username":"newuser","password":"password123"}`)
	w := authenticatedRequest(t, router, http.MethodPost, "/api/v1/users", createBody, cookie)
	assert.Equal(t, http.StatusCreated, w.Code)

	var created api.UserResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	assert.Equal(t, "newuser", created.Username)

	// List users
	w = authenticatedRequest(t, router, http.MethodGet, "/api/v1/users", nil, cookie)
	assert.Equal(t, http.StatusOK, w.Code)

	var list api.PaginatedUsersResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	assert.GreaterOrEqual(t, len(list.Items), 2)

	// Get user
	w = authenticatedRequest(t, router, http.MethodGet, "/api/v1/users/"+itoa(created.ID), nil, cookie)
	assert.Equal(t, http.StatusOK, w.Code)

	var fetched api.UserResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &fetched))
	assert.Equal(t, created.ID, fetched.ID)

	// Update user
	updateBody := []byte(`{"username":"updateduser","password":"newpassword1"}`)
	w = authenticatedRequest(t, router, http.MethodPut, "/api/v1/users/"+itoa(created.ID), updateBody, cookie)
	assert.Equal(t, http.StatusOK, w.Code)

	var updated api.UserResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &updated))
	assert.Equal(t, "updateduser", updated.Username)

	// Delete user
	w = authenticatedRequest(t, router, http.MethodDelete, "/api/v1/users/"+itoa(created.ID), nil, cookie)
	assert.Equal(t, http.StatusNoContent, w.Code)

	w = authenticatedRequest(t, router, http.MethodGet, "/api/v1/users/"+itoa(created.ID), nil, cookie)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUsersCreateValidation(t *testing.T) {
	router := setupTestRouter(t)
	seedAdminUser(t)
	cookie := apiLogin(t, router, "admin", "password123")

	w := authenticatedRequest(t, router, http.MethodPost, "/api/v1/users",
		[]byte(`{"username":"ab","password":"short"}`), cookie)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFeedsCRUD(t *testing.T) {
	router := setupTestRouter(t)
	seedAdminUser(t)
	cookie := apiLogin(t, router, "admin", "password123")

	createBody := []byte(`{"url":"https://example.com/feed.xml"}`)
	w := authenticatedRequest(t, router, http.MethodPost, "/api/v1/feeds", createBody, cookie)
	assert.Equal(t, http.StatusCreated, w.Code)

	var created api.FeedResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	assert.Equal(t, "https://example.com/feed.xml", created.URL)

	w = authenticatedRequest(t, router, http.MethodGet, "/api/v1/feeds", nil, cookie)
	assert.Equal(t, http.StatusOK, w.Code)

	var list api.PaginatedFeedsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	assert.Len(t, list.Items, 1)

	w = authenticatedRequest(t, router, http.MethodGet, "/api/v1/feeds/"+itoa(created.ID), nil, cookie)
	assert.Equal(t, http.StatusOK, w.Code)

	updateBody := []byte(`{"url":"https://example.com/updated.xml"}`)
	w = authenticatedRequest(t, router, http.MethodPut, "/api/v1/feeds/"+itoa(created.ID), updateBody, cookie)
	assert.Equal(t, http.StatusOK, w.Code)

	var updated api.FeedResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &updated))
	assert.Equal(t, "https://example.com/updated.xml", updated.URL)

	w = authenticatedRequest(t, router, http.MethodDelete, "/api/v1/feeds/"+itoa(created.ID), nil, cookie)
	assert.Equal(t, http.StatusNoContent, w.Code)

	w = authenticatedRequest(t, router, http.MethodGet, "/api/v1/feeds/"+itoa(created.ID), nil, cookie)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestFeedsCreateDuplicateURL(t *testing.T) {
	router := setupTestRouter(t)
	seedAdminUser(t)
	cookie := apiLogin(t, router, "admin", "password123")

	body := []byte(`{"url":"https://example.com/feed.xml"}`)
	w := authenticatedRequest(t, router, http.MethodPost, "/api/v1/feeds", body, cookie)
	assert.Equal(t, http.StatusCreated, w.Code)

	w = authenticatedRequest(t, router, http.MethodPost, "/api/v1/feeds", body, cookie)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestItemsListAndGet(t *testing.T) {
	router := setupTestRouter(t)
	seedAdminUser(t)
	cookie := apiLogin(t, router, "admin", "password123")

	feed := models.Feed{URL: "https://example.com/feed.xml", Title: "Test Feed"}
	require.NoError(t, database.DB.Create(&feed).Error)

	item := models.Item{
		FeedID: feed.ID,
		Title:  "Test Article",
		Link:   "https://example.com/article",
		GUID:   "guid-1",
	}
	require.NoError(t, database.DB.Create(&item).Error)

	w := authenticatedRequest(t, router, http.MethodGet, "/api/v1/items", nil, cookie)
	assert.Equal(t, http.StatusOK, w.Code)

	var list api.PaginatedItemsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	assert.Len(t, list.Items, 1)
	assert.Equal(t, "Test Article", list.Items[0].Title)
	assert.NotNil(t, list.Items[0].Feed)

	w = authenticatedRequest(t, router, http.MethodGet, "/api/v1/items?feed_id="+itoa(feed.ID), nil, cookie)
	assert.Equal(t, http.StatusOK, w.Code)

	w = authenticatedRequest(t, router, http.MethodGet, "/api/v1/items/"+itoa(item.ID), nil, cookie)
	assert.Equal(t, http.StatusOK, w.Code)

	var fetched api.ItemResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &fetched))
	assert.Equal(t, item.ID, fetched.ID)
	assert.Equal(t, feed.ID, fetched.FeedID)
}

func TestItemsNotFound(t *testing.T) {
	router := setupTestRouter(t)
	seedAdminUser(t)
	cookie := apiLogin(t, router, "admin", "password123")

	w := authenticatedRequest(t, router, http.MethodGet, "/api/v1/items/999", nil, cookie)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func itoa(n uint) string {
	return strconv.FormatUint(uint64(n), 10)
}
