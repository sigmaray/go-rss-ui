package main

import (
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Username string `gorm:"unique_index"`
	Password string
}

// BeforeSave is a GORM hook to hash password before saving
func (user *User) BeforeSave(tx *gorm.DB) error {
	// Skip hashing if password is empty
	if len(user.Password) == 0 {
		return nil
	}
	
	// Check if password is already a bcrypt hash
	// Bcrypt hashes start with $2a$, $2b$, or $2y$ and are 60 characters long
	isHashed := len(user.Password) == 60 && 
		len(user.Password) >= 4 && 
		user.Password[0] == '$' && 
		user.Password[1] == '2' && 
		(user.Password[2] == 'a' || user.Password[2] == 'b' || user.Password[2] == 'y') && 
		user.Password[3] == '$'
	
	// Only hash if password is not already hashed
	// This prevents double-hashing when updating users without changing password
	if !isHashed {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		user.Password = string(hashedPassword)
	}
	return nil
}

// CheckPassword compares a plaintext password with the user's hashed password
func (user *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	return err == nil
}

type Feed struct {
	gorm.Model
	URL         string `gorm:"unique_index;not null"`
	Title       string
	Description string
	Items       []Item `gorm:"foreignKey:FeedID"`
}

type Item struct {
	gorm.Model
	FeedID      uint   `gorm:"not null;index"`
	Title       string
	Link        string
	Description string `gorm:"type:text"`
	Content     string `gorm:"type:text"`
	Author      string
	PublishedAt *time.Time
	GUID        string `gorm:"index"` // Unique identifier from feed
	Feed        Feed   `gorm:"foreignKey:FeedID"`
}

