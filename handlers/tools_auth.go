package handlers

import (
	"log"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"go-rss-ui/services"
)

func SeedUsers(c *gin.Context) {
	if !requireCypressMode(c) {
		return
	}

	session := sessions.Default(c)
	result, err := services.EnsureAdminUser()
	if err != nil {
		addFlashError(session, "Failed to ensure admin user: "+err.Error())
		saveSession(session)
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	if result.Created {
		addFlashSuccess(session, "Admin user 'admin' created with password 'password'")
	} else {
		addFlashSuccess(session, "Admin user already exists")
	}

	saveSession(session)
	c.Redirect(http.StatusFound, "/tools")
}

func SeedUsersAndLogin(c *gin.Context) {
	if !requireCypressMode(c) {
		return
	}

	session := sessions.Default(c)
	result, err := services.EnsureAdminUser()
	if err != nil {
		addFlashError(session, "Failed to ensure admin user: "+err.Error())
		saveSession(session)
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	session.Set("user", result.User.ID)
	if err := session.Save(); err != nil {
		log.Printf("Error saving session in SeedUsersAndLogin: %v", err)
		addFlashError(session, "Failed to save session")
		saveSession(session)
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	if result.Created {
		addFlashSuccess(session, "Admin user created and logged in successfully")
	} else {
		addFlashSuccess(session, "Logged in as admin successfully")
	}
	saveSession(session)
	c.Redirect(http.StatusFound, "/admin/users")
}
