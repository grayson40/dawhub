package web

import (
	"net/http"

	"dawhub/internal/domain"
	"dawhub/internal/repository"
	"dawhub/pkg/common"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	userRepo *repository.UserRepository
}

func NewAuthHandler(userRepo *repository.UserRepository) *AuthHandler {
	return &AuthHandler{userRepo: userRepo}
}

func (h *AuthHandler) LoginPage(c *gin.Context) {
	c.HTML(http.StatusOK, "auth_layout", gin.H{
		"content": "login",
	})
}

func (h *AuthHandler) RegisterPage(c *gin.Context) {
	c.HTML(http.StatusOK, "auth_layout", gin.H{
		"content": "register",
	})
}

func (h *AuthHandler) Register(c *gin.Context) {
	var user domain.User
	if err := c.ShouldBind(&user); err != nil {
		c.HTML(http.StatusBadRequest, "auth_layout", gin.H{
			"content": "register",
			"error":   err.Error(),
		})
		return
	}

	if user.Username == "" || user.Email == "" || user.Password == "" {
		c.HTML(http.StatusBadRequest, "auth_layout", gin.H{
			"content": "register",
			"error":   "All fields are required",
		})
		return
	}

	if err := user.HashPassword(); err != nil {
		c.HTML(http.StatusInternalServerError, "auth_layout", gin.H{
			"content": "register",
			"error":   "Server error",
		})
		return
	}

	if err := h.userRepo.Create(&user); err != nil {
		c.HTML(http.StatusInternalServerError, "auth_layout", gin.H{
			"content": "register",
			"error":   "Failed to create user",
		})
		return
	}

	// Store user data in session
	session := sessions.Default(c)
	session.Set("user_id", user.ID)
	session.Set("username", user.Username)
	session.Set("email", user.Email)
	session.Save()

	c.Redirect(http.StatusSeeOther, "/")
}

func (h *AuthHandler) Login(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	user, err := h.userRepo.GetByUsername(username)
	if err != nil {
		c.HTML(http.StatusUnauthorized, "auth_layout", gin.H{
			"content": "login",
			"error":   "Invalid credentials",
		})
		return
	}

	if !user.CheckPassword(password) {
		c.HTML(http.StatusUnauthorized, "auth_layout", gin.H{
			"content": "login",
			"error":   "Invalid credentials",
		})
		return
	}

	// Store user data in session
	session := sessions.Default(c)
	session.Set("user_id", user.ID)
	session.Set("username", user.Username)
	session.Set("email", user.Email)
	session.Save()

	c.Redirect(http.StatusSeeOther, "/")
}

func (h *AuthHandler) Logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	session.Save()

	c.Redirect(http.StatusSeeOther, "/login")
}

func (h *AuthHandler) SettingsPage(c *gin.Context) {
	common.Render(c, gin.H{
		"content": "settings",
	})
}

func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	// Handle profile update
}

func (h *AuthHandler) UpdatePassword(c *gin.Context) {
	// Handle password update
}

func (h *AuthHandler) UpdatePreferences(c *gin.Context) {
	// Handle preferences update
}

func (h *AuthHandler) DeleteAccount(c *gin.Context) {
	// Handle account deletion
}
