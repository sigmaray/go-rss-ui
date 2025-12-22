package main

import (
	"log"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func main() {
	ConnectDatabase()
	seedUser()

	r := gin.Default()
	r.LoadHTMLGlob("templates/*")

	store := cookie.NewStore([]byte("secret"))
	r.Use(sessions.Sessions("mysession", store))

	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"title": "My RSS App",
		})
	})

	admin := r.Group("/admin")
	{
		admin.Use(AuthRequired())
		admin.GET("/", adminIndex)
		admin.POST("/users", createUser)
		admin.POST("/users/:id/edit", editUser)
		admin.POST("/users/:id/delete", deleteUser)
	}

	r.GET("/login", showLogin)
	r.POST("/login", login)
	r.POST("/logout", logout)

	r.Run(":8081")
}

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		user := session.Get("user")
		if user == nil {
			log.Println("User not logged in")
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		c.Next()
	}
}

func seedUser() {
	var user User
	result := DB.Where("username = ?", "admin").First(&user)
	if result.Error == gorm.ErrRecordNotFound {
		adminUser := User{Username: "admin", Password: "password"}
		if err := DB.Create(&adminUser).Error; err != nil {
			log.Fatalf("Failed to create admin user: %v", err)
		}
		log.Println("Admin user 'admin' created with password 'password'")
	}
}

func showLogin(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", gin.H{
		"title": "Login",
	})
}

func login(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	var user User
	result := DB.Where("username = ?", username).First(&user)
	if result.Error != nil {
		c.HTML(http.StatusUnauthorized, "login.html", gin.H{
			"title":   "Login",
			"message": "Invalid credentials",
		})
		return
	}

	if !user.CheckPassword(password) {
		c.HTML(http.StatusUnauthorized, "login.html", gin.H{
			"title":   "Login",
			"message": "Invalid credentials",
		})
		return
	}

	session := sessions.Default(c)
	session.Set("user", user.Username)
	if err := session.Save(); err != nil {
		log.Printf("Error saving session: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save session"})
		return
	}

	c.Redirect(http.StatusFound, "/admin")
}

func logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	session.Save()
	c.Redirect(http.StatusFound, "/")
}

func adminIndex(c *gin.Context) {
	var users []User
	DB.Find(&users)

	c.HTML(http.StatusOK, "admin.html", gin.H{
		"title": "Admin Panel",
		"users": users,
	})
}

func createUser(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	if username == "" || password == "" {
		c.HTML(http.StatusBadRequest, "admin.html", gin.H{
			"title": "Admin Panel",
			"error": "Username and password are required",
			"users": []User{},
		})
		return
	}

	user := User{Username: username, Password: password}
	if err := DB.Create(&user).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "admin.html", gin.H{
			"title": "Admin Panel",
			"error": "Failed to create user: " + err.Error(),
			"users": []User{},
		})
		return
	}

	c.Redirect(http.StatusFound, "/admin")
}

func editUser(c *gin.Context) {
	id := c.Param("id")
	username := c.PostForm("username")
	password := c.PostForm("password")

	var user User
	if err := DB.First(&user, id).Error; err != nil {
		c.HTML(http.StatusNotFound, "admin.html", gin.H{
			"title": "Admin Panel",
			"error": "User not found",
			"users": []User{},
		})
		return
	}

	if username != "" {
		user.Username = username
	}
	if password != "" {
		user.Password = password
	}

	if err := DB.Save(&user).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "admin.html", gin.H{
			"title": "Admin Panel",
			"error": "Failed to update user: " + err.Error(),
			"users": []User{},
		})
		return
	}

	c.Redirect(http.StatusFound, "/admin")
}

func deleteUser(c *gin.Context) {
	id := c.Param("id")

	var user User
	if err := DB.First(&user, id).Error; err != nil {
		c.HTML(http.StatusNotFound, "admin.html", gin.H{
			"title": "Admin Panel",
			"error": "User not found",
			"users": []User{},
		})
		return
	}

	if err := DB.Delete(&user).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "admin.html", gin.H{
			"title": "Admin Panel",
			"error": "Failed to delete user: " + err.Error(),
			"users": []User{},
		})
		return
	}

	c.Redirect(http.StatusFound, "/admin")
}
