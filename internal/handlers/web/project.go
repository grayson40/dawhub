package web

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"dawhub/internal/domain"
)

// Stats represents the dashboard statistics
type Stats struct {
	ProjectCount int
	SampleCount  int
	UserCount    int
}

// HomeData represents all data needed for the home page
type HomeData struct {
	Stats            Stats
	TrendingProjects []domain.Project
	RecentProjects   []domain.Project
}

// ProjectHandler handles web interface requests for project operations
type ProjectHandler struct {
	repo    domain.ProjectRepository
	storage domain.StorageService
}

// NewProjectHandler creates a new web project handler instance
func NewProjectHandler(repo domain.ProjectRepository, storage domain.StorageService) *ProjectHandler {
	return &ProjectHandler{
		repo:    repo,
		storage: storage,
	}
}

// Home handles GET / to display the dashboard
func (h *ProjectHandler) Home(c *gin.Context) {
	// Get dashboard data
	data, err := h.getHomeData()
	if err != nil {
		h.renderError(c, "Failed to load dashboard")
		return
	}

	c.HTML(http.StatusOK, "base", gin.H{
		"content":          "index",
		"stats":            data.Stats,
		"trendingProjects": data.TrendingProjects,
		"recentProjects":   data.RecentProjects,
	})
}

// getHomeData collects all data needed for the home page
func (h *ProjectHandler) getHomeData() (HomeData, error) {
	var data HomeData

	// Get all projects for stats
	projects, err := h.repo.FindAll()
	if err != nil {
		return data, err
	}

	// Calculate stats
	data.Stats = Stats{
		ProjectCount: len(projects),
		// TODO: Implement sample and user counts when those features are added
	}

	// Get trending projects (newest for now)
	if len(projects) > 6 {
		data.TrendingProjects = projects[len(projects)-6:]
	} else {
		data.TrendingProjects = projects
	}

	// Get recent projects
	if len(projects) > 4 {
		data.RecentProjects = projects[len(projects)-4:]
	} else {
		data.RecentProjects = projects
	}

	return data, nil
}

// List handles GET / to display all projects
func (h *ProjectHandler) List(c *gin.Context) {
	projects, err := h.repo.FindAll()
	if err != nil {
		h.renderError(c, "Failed to fetch projects")
		return
	}

	c.HTML(http.StatusOK, "base", gin.H{
		"content":  "projects",
		"projects": projects,
	})
}

// New handles GET /projects/new to display project creation form
func (h *ProjectHandler) New(c *gin.Context) {
	c.HTML(http.StatusOK, "base", gin.H{
		"content": "new",
	})
}

// Create handles POST /projects/create to create a new project
func (h *ProjectHandler) Create(c *gin.Context) {
	project := &domain.Project{
		Name:    c.PostForm("name"),
		Version: c.PostForm("version"),
	}

	if err := h.repo.Create(project); err != nil {
		h.renderError(c, "Failed to create project")
		return
	}

	redirectUrl := fmt.Sprintf("/projects/%d", project.ID)
	c.Redirect(http.StatusFound, redirectUrl)
}

// Show handles GET /projects/:id to display a specific project
func (h *ProjectHandler) Show(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		h.renderError(c, "Invalid project ID")
		return
	}

	project, err := h.repo.FindByID(uint(id))
	if err != nil {
		h.renderError(c, "Project not found")
		return
	}

	c.HTML(http.StatusOK, "base", gin.H{
		"content": "show",
		"project": project,
	})
}

// Edit handles GET /projects/:id/edit to display project edit form
func (h *ProjectHandler) Edit(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		h.renderError(c, "Invalid project ID")
		return
	}

	project, err := h.repo.FindByID(uint(id))
	if err != nil {
		h.renderError(c, "Project not found")
		return
	}

	c.HTML(http.StatusOK, "base", gin.H{
		"content": "edit",
		"project": project,
	})
}

// Update handles POST /projects/:id/update to modify existing project
func (h *ProjectHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		h.renderError(c, "Invalid project ID")
		return
	}

	project, err := h.repo.FindByID(uint(id))
	if err != nil {
		h.renderError(c, "Project not found")
		return
	}

	project.Name = c.PostForm("name")
	project.Version = c.PostForm("version")

	if err := h.repo.Update(project); err != nil {
		h.renderError(c, "Failed to update project")
		return
	}

	c.Redirect(http.StatusFound, fmt.Sprintf("/projects/%d", project.ID))
}

// Delete handles POST /projects/:id/delete to remove a project
func (h *ProjectHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		h.renderError(c, "Invalid project ID")
		return
	}

	// Get project before delete
	project, err := h.repo.FindByID(uint(id))
	if err != nil {
		h.renderError(c, "Project not found")
		return
	}

	if err := h.repo.Delete(uint(id)); err != nil {
		h.renderError(c, "Failed to delete project")
		return
	}

	if err := h.storage.DeleteFile(project.FilePath); err != nil {
		h.renderError(c, "Failed to delete project file")
		return
	}

	c.Redirect(http.StatusFound, "/projects")
}

// renderError renders the error template with given message
func (h *ProjectHandler) renderError(c *gin.Context, message string) {
	c.HTML(http.StatusInternalServerError, "base", gin.H{
		"content": "error",
		"error":   message,
	})
}
