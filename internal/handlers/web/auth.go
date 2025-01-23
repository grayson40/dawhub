package web

import (
	"log"
	"math"
	"net/http"
	"strconv"

	"dawhub/internal/domain"
	"dawhub/internal/email"
	"dawhub/internal/repository"
	"dawhub/pkg/common"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	userRepo     *repository.UserRepository
	projectRepo  domain.ProjectRepository
	emailService *email.ResendService
}

func NewAuthHandler(userRepo *repository.UserRepository, projectRepo domain.ProjectRepository, emailService *email.ResendService) *AuthHandler {
	return &AuthHandler{
		userRepo:     userRepo,
		projectRepo:  projectRepo,
		emailService: emailService,
	}
}

// Landing route
func (h *AuthHandler) LandingPage(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")

	c.HTML(http.StatusOK, "landing", gin.H{
		"content":    "landing",
		"isLoggedIn": userID != nil,
	})
}

// Health route
func (h *AuthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
	})
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

	c.Redirect(http.StatusSeeOther, "/dashboard")
}

func (h *AuthHandler) Logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	session.Save()

	c.Redirect(http.StatusSeeOther, "/")
}

func (h *AuthHandler) SettingsPage(c *gin.Context) {
	common.Render(c, gin.H{
		"content": "settings",
	})
}

func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	sessionUsername := session.Get("username").(string)
	sessionEmail := session.Get("email").(string)

	if userID == nil {
		c.Header("HX-Trigger", `{"showToast": {"message": "User must be logged in", "type": "error"}}`)
		return
	}

	user, err := h.userRepo.GetByID(uint(userID.(uint)))
	if err != nil {
		c.Header("HX-Trigger", `{"showToast": {"message": "User not found", "type": "error"}}`)
		return
	}

	username := c.PostForm("username")
	email := c.PostForm("email")

	if username == "" || email == "" {
		c.Header("HX-Trigger", `{"showToast": {"message": "All fields are required", "type": "error"}}`)
		return
	}

	// Check if anything changed
	if username == sessionUsername && email == sessionEmail {
		return
	}

	// Only update if there are changes
	if username != sessionUsername || email != sessionEmail {
		user.Username = username
		user.Email = email

		if err := h.userRepo.Update(user); err != nil {
			c.Header("HX-Trigger", `{"showToast": {"message": "Failed to update profile", "type": "error"}}`)
			return
		}

		session.Set("username", username)
		session.Set("email", email)
		session.Save()
	}

	c.Header("HX-Trigger", `{"showToast": {"message": "Profile updated successfully", "type": "success"}}`)
	c.Status(http.StatusOK)
}

func (h *AuthHandler) UpdatePassword(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	if userID == nil {
		c.Header("HX-Trigger", `{"showToast": {"message": "User must be logged in", "type": "error"}}`)
		return
	}

	user, err := h.userRepo.GetByID(uint(userID.(uint)))
	if err != nil {
		c.Header("HX-Trigger", `{"showToast": {"message": "User not found", "type": "error"}}`)
		return
	}

	currentPassword := c.PostForm("current_password")
	newPassword := c.PostForm("new_password")

	if currentPassword == "" || newPassword == "" {
		c.Header("HX-Trigger", `{"showToast": {"message": "All fields are required", "type": "error"}}`)
		return
	}

	if !user.CheckPassword(currentPassword) {
		c.Header("HX-Trigger", `{"showToast": {"message": "Current password is incorrect", "type": "error"}}`)
		return
	}

	user.Password = newPassword
	if err := user.HashPassword(); err != nil {
		c.Header("HX-Trigger", `{"showToast": {"message": "Failed to update password", "type": "error"}}`)
		return
	}

	if err := h.userRepo.Update(user); err != nil {
		c.Header("HX-Trigger", `{"showToast": {"message": "Failed to update password", "type": "error"}}`)
		return
	}

	// Clear form inputs using HX-Reswap header
	c.Header("HX-Trigger", `{
		"showToast": {"message": "Password updated successfully", "type": "success"},
		"clearPasswords": true
	}`)
	c.Status(http.StatusOK)
}

func (h *AuthHandler) DeleteAccount(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	if userID == nil {
		c.Header("HX-Trigger", `{"showToast": {"message": "User must be logged in", "type": "error"}}`)
		return
	}

	// Get user projects
	projects, err := h.projectRepo.FindByUserID(uint(userID.(uint)))
	if err != nil {
		c.Header("HX-Trigger", `{"showToast": {"message": "Failed to get user projects", "type": "error"}}`)
		return
	}

	// Delete all user projects
	for _, project := range projects {
		if err := h.projectRepo.Delete(project.ID); err != nil {
			c.Header("HX-Trigger", `{"showToast": {"message": "Failed to delete user projects", "type": "error"}}`)
			return
		}
	}

	// Delete user
	if err := h.userRepo.Delete(uint(userID.(uint))); err != nil {
		c.Header("HX-Trigger", `{"showToast": {"message": "Failed to delete account", "type": "error"}}`)
		return
	}

	session.Clear()
	session.Save()

	c.Header("HX-Trigger", `{"showToast": {"message": "Account deleted successfully", "type": "success"}}`)
	c.Header("HX-Redirect", "/")
	c.Status(http.StatusOK)
}

// Add this function to handle beta signup
func (h *AuthHandler) BetaSignup(c *gin.Context) {
	var betaUser domain.BetaUser
	if err := c.ShouldBind(&betaUser); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	// Check if the email is already registered
	existingUser, err := h.userRepo.GetBetaUserByEmail(betaUser.Email)
	if err == nil && existingUser != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already registered"})
		return
	}

	// Save the user to the database
	if err := h.userRepo.CreateBetaUser(&betaUser); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Send welcome email
	err = h.emailService.SendBetaSignupEmail(betaUser.Email)
	if err != nil {
		log.Printf("Failed to send welcome email: %v", err)
		// Continue execution - don't return error to user
	}

	// Return success message
	c.JSON(http.StatusOK, gin.H{"message": "Signup successful"})
}

// Update the BetaUsersPage function to handle pagination
func (h *AuthHandler) BetaUsersPage(c *gin.Context) {
	pageStr := c.Query("page")
	limitStr := c.Query("limit")

	page := 1   // Default to page 1
	limit := 10 // Default to 10 items per page

	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil {
			page = p
		}
	}
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	betaUsers, err := h.userRepo.GetAllBetaUsers(page, limit)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"message": "Failed to fetch beta users"})
		return
	}

	// Get total count for pagination
	totalCount := int64(0)
	if err := h.userRepo.CountBetaUsers(&totalCount); err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"message": "Failed to count beta users"})
		return
	}

	totalPages := int(math.Ceil(float64(totalCount) / float64(limit)))

	c.HTML(http.StatusOK, "beta_users", gin.H{
		"betaUsers":   betaUsers,
		"currentPage": page,
		"totalPages":  totalPages,
	})
}
