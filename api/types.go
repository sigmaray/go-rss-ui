package api

import (
	"time"

	"go-rss-ui/models"
)

type ErrorResponse struct {
	Error string `json:"error" example:"resource not found"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required" example:"admin"`
	Password string `json:"password" binding:"required" example:"password"`
}

type MessageResponse struct {
	Message string `json:"message" example:"logged out"`
}

type UserResponse struct {
	ID        uint      `json:"id" example:"1"`
	Username  string    `json:"username" example:"admin"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type FeedResponse struct {
	ID                        uint       `json:"id" example:"1"`
	URL                       string     `json:"url" example:"https://example.com/feed.xml"`
	Title                     string     `json:"title" example:"Example Feed"`
	Description               string     `json:"description"`
	LastSuccessfullyFetchedAt *time.Time `json:"last_successfully_fetched_at"`
	LastError                 string     `json:"last_error"`
	LastErrorAt               *time.Time `json:"last_error_at"`
	CreatedAt                 time.Time  `json:"created_at"`
	UpdatedAt                 time.Time  `json:"updated_at"`
}

type ItemResponse struct {
	ID          uint         `json:"id" example:"1"`
	FeedID      uint         `json:"feed_id" example:"1"`
	Title       string       `json:"title" example:"Article title"`
	Link        string       `json:"link" example:"https://example.com/article"`
	Description string       `json:"description"`
	Content     string       `json:"content"`
	Author      string       `json:"author"`
	PublishedAt *time.Time   `json:"published_at"`
	GUID        string       `json:"guid"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	Feed        *FeedSummary `json:"feed,omitempty"`
}

type FeedSummary struct {
	ID    uint   `json:"id" example:"1"`
	URL   string `json:"url" example:"https://example.com/feed.xml"`
	Title string `json:"title" example:"Example Feed"`
}

type PaginatedUsersResponse struct {
	Page       int64          `json:"page" example:"1"`
	TotalPages int64          `json:"total_pages" example:"1"`
	Total      int64          `json:"total" example:"1"`
	Items      []UserResponse `json:"items"`
}

type PaginatedFeedsResponse struct {
	Page       int64          `json:"page" example:"1"`
	TotalPages int64          `json:"total_pages" example:"1"`
	Total      int64          `json:"total" example:"1"`
	Items      []FeedResponse `json:"items"`
}

type PaginatedItemsResponse struct {
	Page       int64          `json:"page" example:"1"`
	TotalPages int64          `json:"total_pages" example:"1"`
	Total      int64          `json:"total" example:"1"`
	Items      []ItemResponse `json:"items"`
}

type FetchFeedResponse struct {
	ItemsCreated int `json:"items_created" example:"5"`
	ItemsUpdated int `json:"items_updated" example:"2"`
}

func toUserResponse(user models.User) UserResponse {
	return UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

func toFeedResponse(feed models.Feed) FeedResponse {
	return FeedResponse{
		ID:                        feed.ID,
		URL:                       feed.URL,
		Title:                     feed.Title,
		Description:               feed.Description,
		LastSuccessfullyFetchedAt: feed.LastSuccessfullyFetchedAt,
		LastError:                 feed.LastError,
		LastErrorAt:               feed.LastErrorAt,
		CreatedAt:                 feed.CreatedAt,
		UpdatedAt:                 feed.UpdatedAt,
	}
}

func toItemResponse(item models.Item, includeFeed bool) ItemResponse {
	resp := ItemResponse{
		ID:          item.ID,
		FeedID:      item.FeedID,
		Title:       item.Title,
		Link:        item.Link,
		Description: item.Description,
		Content:     item.Content,
		Author:      item.Author,
		PublishedAt: item.PublishedAt,
		GUID:        item.GUID,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
	if includeFeed && item.Feed.ID != 0 {
		resp.Feed = &FeedSummary{
			ID:    item.Feed.ID,
			URL:   item.Feed.URL,
			Title: item.Feed.Title,
		}
	}
	return resp
}
