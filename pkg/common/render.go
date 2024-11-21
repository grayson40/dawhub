package common

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// Handler provides common functionality for web handlers
type Handler struct{}

// IsHtmx checks if the request is coming from HTMX
func IsHtmx(c *gin.Context) bool {
	return c.GetHeader("HX-Request") == "true"
}

// Render handles template rendering for both HTMX and regular requests
func Render(c *gin.Context, data gin.H) {
	session := sessions.Default(c)
	if userID := session.Get("user_id"); userID != nil {
		data["user"] = gin.H{
			"ID":       userID,
			"Username": session.Get("username"),
			"Email":    session.Get("email"),
		}
	}

	if IsHtmx(c) {
		c.HTML(http.StatusOK, "content", data)
	} else {
		c.HTML(http.StatusOK, "base", data)
	}
}

// RenderError renders an error response
func RenderError(c *gin.Context, message string) {
	if IsHtmx(c) {
		c.HTML(http.StatusBadRequest, "error", gin.H{
			"error": message,
		})
		return
	}

	// For non-HTMX requests
	c.HTML(http.StatusBadRequest, "base", gin.H{
		"content": "error",
		"error":   message,
	})
}

// HandleRedirect handles redirects for both HTMX and regular requests
func HandleRedirect(c *gin.Context, redirectUrl string) {
	if IsHtmx(c) {
		c.Header("HX-Redirect", redirectUrl)
		c.Status(http.StatusOK)
	} else {
		c.Redirect(http.StatusFound, redirectUrl)
	}
}
