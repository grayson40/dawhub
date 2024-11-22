package web

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"

	"dawhub/internal/domain"
	"dawhub/pkg/common"
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
	data, err := h.getHomeData()
	if err != nil {
		common.RenderError(c, "Failed to load dashboard")
		return
	}

	common.Render(c, gin.H{
		"content":          "index",
		"stats":            data.Stats,
		"trendingProjects": data.TrendingProjects,
		"recentProjects":   data.RecentProjects,
	})
}

// getHomeData collects all data needed for the home page
func (h *ProjectHandler) getHomeData() (HomeData, error) {
	var data HomeData

	// Get public projects only
	projects, err := h.repo.FindAllPublic()
	if err != nil {
		return data, fmt.Errorf("failed to fetch projects: %v", err)
	}

	data.Stats = Stats{
		ProjectCount: len(projects),
	}

	// Get last 6 trending projects
	if len(projects) > 6 {
		data.TrendingProjects = projects[:6]
	} else {
		data.TrendingProjects = projects
	}

	// Get last 4 recent projects
	if len(projects) > 4 {
		data.RecentProjects = projects[:4]
	} else {
		data.RecentProjects = projects
	}

	return data, nil
}

// List handles GET / to display all projects
func (h *ProjectHandler) List(c *gin.Context) {
	// Get logged in user ID
	session := sessions.Default(c)
	userID := session.Get("user_id")
	if userID == nil {
		common.RenderError(c, "User must be logged in")
		return
	}

	// Fetch only user's projects
	projects, err := h.repo.FindByUserID(userID.(uint))
	if err != nil {
		common.RenderError(c, "Failed to fetch projects")
		return
	}

	common.Render(c, gin.H{
		"content":  "projects",
		"projects": projects,
	})
}

// New handles GET /projects/new to display project creation form
func (h *ProjectHandler) New(c *gin.Context) {
	common.Render(c, gin.H{
		"content": "new",
	})
}

// Create handles POST /projects/create to create a new project with files
func (h *ProjectHandler) Create(c *gin.Context) {
	// Get logged in user ID
	session := sessions.Default(c)
	userID := session.Get("user_id")
	if userID == nil {
		common.RenderError(c, "User must be logged in")
		return
	}

	// Parse form data
	form, err := c.MultipartForm()
	if err != nil {
		common.RenderError(c, "Invalid form data")
		return
	}

	// Start transaction
	tx, err := h.repo.Begin()
	if err != nil {
		common.RenderError(c, "Failed to start transaction")
		return
	}
	defer tx.Rollback()

	// Create initial project
	project := &domain.Project{
		Name:        c.PostForm("name"),
		Description: c.PostForm("description"),
		IsPublic:    c.PostForm("visibility") == "public",
		UserID:      userID.(uint),
		Version:     "1.0",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Create project first to get ID
	if err := tx.Create(project); err != nil {
		common.RenderError(c, "Failed to create project")
		return
	}

	// Handle main project file
	mainFile, err := c.FormFile("mainFile")
	if err != nil {
		common.RenderError(c, "Main project file is required")
		return
	}

	file, err := mainFile.Open()
	if err != nil {
		common.RenderError(c, "Failed to read main file")
		return
	}
	defer file.Close()

	// Upload main file
	fileInfo, filePath, err := h.storage.UploadFile(project.ID, mainFile.Filename, file)
	if err != nil {
		common.RenderError(c, "Failed to upload main file")
		return
	}

	// Create ProjectFile record
	projectFile := &domain.ProjectFile{
		FileMetadata: domain.FileMetadata{
			Size:        fileInfo.Size,
			Filename:    fileInfo.Filename,
			ContentType: fileInfo.ContentType,
			Hash:        fileInfo.Hash,
			UploadedAt:  time.Now(),
		},
		FilePath: filePath,
	}

	if err := tx.AddMainFile(project.ID, projectFile); err != nil {
		// Cleanup uploaded file
		h.storage.DeleteFile(filePath)
		common.RenderError(c, "Failed to save main file info")
		return
	}

	// Handle sample files if any
	if sampleFiles := form.File["samples"]; len(sampleFiles) > 0 {
		for _, sampleFile := range sampleFiles {
			file, err := sampleFile.Open()
			if err != nil {
				continue
			}
			defer file.Close()

			fileInfo, filePath, err := h.storage.UploadFile(project.ID, sampleFile.Filename, file)
			if err != nil {
				continue
			}

			sample := &domain.SampleFile{
				ProjectID: project.ID,
				FileMetadata: domain.FileMetadata{
					Size:        fileInfo.Size,
					Filename:    fileInfo.Filename,
					ContentType: fileInfo.ContentType,
					Hash:        fileInfo.Hash,
					UploadedAt:  time.Now(),
				},
				FilePath: filePath,
			}

			if err := tx.AddSampleFile(project.ID, sample); err != nil {
				// Cleanup uploaded file
				h.storage.DeleteFile(filePath)
				continue
			}
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		common.RenderError(c, "Failed to save project")
		return
	}

	// Return success response for HTMX
	if c.GetHeader("HX-Request") == "true" {
		c.HTML(200, "project-created", gin.H{
			"project": project,
			"success": true,
			"message": "Project created successfully",
		})
		return
	}

	// Regular redirect for non-HTMX requests
	redirectUrl := fmt.Sprintf("/projects/%d", project.ID)
	common.HandleRedirect(c, redirectUrl)
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

	common.Render(c, gin.H{
		"content":        "show",
		"project":        project,
		"formatFileSize": formatFileSize,
	})
}

// Edit handles GET /projects/:id/edit to display project edit form
func (h *ProjectHandler) Edit(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.RenderError(c, "Invalid project ID")
		return
	}

	project, err := h.repo.FindByID(uint(id))
	if err != nil {
		common.RenderError(c, "Project not found")
		return
	}

	common.Render(c, gin.H{
		"content": "edit",
		"project": project,
	})
}

// Update handles POST /projects/:id/update to modify existing project
func (h *ProjectHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.RenderError(c, "Invalid project ID")
		return
	}

	project, err := h.repo.FindByID(uint(id))
	if err != nil {
		common.RenderError(c, "Project not found")
		return
	}

	project.Name = c.PostForm("name")
	project.Description = c.PostForm("description")
	project.Version = c.PostForm("version")
	project.IsPublic = c.PostForm("visibility") == "public"

	if err := h.repo.Update(project); err != nil {
		common.RenderError(c, "Failed to update project")
		return
	}

	// After successful update, redirect to the show page
	redirectUrl := fmt.Sprintf("/projects/%d", project.ID)
	common.HandleRedirect(c, redirectUrl)
}

// Delete handles POST /projects/:id/delete to remove a project
func (h *ProjectHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.RenderError(c, "Invalid project ID")
		return
	}

	// Start transaction
	tx, err := h.repo.Begin()
	if err != nil {
		common.RenderError(c, "Failed to start transaction")
		return
	}
	defer tx.Rollback()

	// Get project with all associated files
	project, err := h.repo.FindByID(uint(id))
	if err != nil {
		common.RenderError(c, "Project not found")
		return
	}

	// Delete all sample files from storage first
	for _, sample := range project.SampleFiles {
		if err := h.storage.DeleteFile(sample.FilePath); err != nil {
			log.Printf("Failed to delete sample file %s: %v", sample.FilePath, err)
			// Continue deletion even if one file fails
		}
	}

	// Delete main project file from storage if it exists
	if project.MainFile != nil && project.MainFile.FilePath != "" {
		if err := h.storage.DeleteFile(project.MainFile.FilePath); err != nil {
			log.Printf("Failed to delete main file %s: %v", project.MainFile.FilePath, err)
			// Continue deletion even if main file fails
		}
	}

	// Delete all sample files from database
	if err := tx.RemoveSampleFiles(project.ID, nil); err != nil {
		common.RenderError(c, "Failed to delete sample files")
		return
	}

	// Delete main file from database if it exists
	if project.MainFileID != nil {
		sqlDB := tx.DB() // Get the underlying *gorm.DB
		if err := sqlDB.Exec("UPDATE projects SET main_file_id = NULL WHERE id = ?", project.ID).Error; err != nil {
			common.RenderError(c, "Failed to unlink main file")
			return
		}

		if err := sqlDB.Delete(&domain.ProjectFile{}, *project.MainFileID).Error; err != nil {
			common.RenderError(c, "Failed to delete main file record")
			return
		}
	}

	// Finally delete the project
	if err := tx.Delete(project.ID); err != nil {
		common.RenderError(c, "Failed to delete project")
		return
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		common.RenderError(c, "Failed to complete deletion")
		return
	}

	// For HTMX requests, we'll either redirect or render the projects list
	if common.IsHtmx(c) {
		c.Header("HX-Redirect", "/projects")
		projects, err := h.repo.FindAll()
		if err != nil {
			common.RenderError(c, "Failed to fetch projects")
			return
		}
		common.Render(c, gin.H{
			"content":  "projects",
			"projects": projects,
		})
	} else {
		c.Redirect(http.StatusFound, "/projects")
	}
}

func (h *ProjectHandler) Import(c *gin.Context) {
	common.Render(c, gin.H{
		"content": "import",
	})
}

func (h *ProjectHandler) HandleImport(c *gin.Context) {
	// Get multipart form
	file, _, err := c.Request.FormFile("projectZip")
	if err != nil {
		h.renderError(c, "No file uploaded")
		return
	}
	defer file.Close()

	// Read zip file
	data, err := io.ReadAll(file)
	if err != nil {
		h.renderError(c, "Failed to read zip file")
		return
	}

	// Process zip file
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		h.renderError(c, "Invalid zip file")
		return
	}

	// Find main project file and samples
	var mainFile *zip.File
	var sampleFiles []*zip.File

	for _, f := range reader.File {
		ext := filepath.Ext(f.Name)
		if isProjectFile(ext) {
			mainFile = f
		} else if isAudioFile(ext) {
			sampleFiles = append(sampleFiles, f)
		}
	}

	if mainFile == nil {
		h.renderError(c, "No project file found in zip")
		return
	}

	// Create new project using main file name
	project := &domain.Project{
		Name:        strings.TrimSuffix(mainFile.Name, filepath.Ext(mainFile.Name)), // Use filename without extension
		Description: fmt.Sprintf("Imported project with %d samples", len(sampleFiles)),
		Version:     "1.0",
		IsPublic:    c.PostForm("visibility") == "public",
	}

	// Start transaction
	tx, err := h.repo.Begin()
	if err != nil {
		h.renderError(c, "Failed to start transaction")
		return
	}
	defer tx.Rollback()

	// Create project first to get ID
	if err := tx.Create(project); err != nil {
		h.renderError(c, "Failed to create project")
		return
	}

	// Process and upload the main file
	mainFileContent, err := readZipFile(mainFile)
	if err != nil {
		h.renderError(c, "Failed to read project file")
		return
	}

	fileInfo, filePath, err := h.storage.UploadFile(project.ID, mainFile.Name, bytes.NewReader(mainFileContent))
	if err != nil {
		h.renderError(c, "Failed to upload project file")
		return
	}

	// Create ProjectFile record
	projectFile := &domain.ProjectFile{
		FileMetadata: domain.FileMetadata{
			Size:        fileInfo.Size,
			Filename:    fileInfo.Filename,
			ContentType: fileInfo.ContentType,
			Hash:        fileInfo.Hash,
			UploadedAt:  time.Now(),
		},
		FilePath: filePath,
	}

	if err := tx.AddMainFile(project.ID, projectFile); err != nil {
		h.storage.DeleteFile(filePath)
		h.renderError(c, "Failed to save project file info")
		return
	}

	// Upload sample files
	for _, sampleFile := range sampleFiles {
		content, err := readZipFile(sampleFile)
		if err != nil {
			log.Printf("Failed to read sample file %s: %v", sampleFile.Name, err)
			continue
		}

		fileInfo, filePath, err := h.storage.UploadFile(project.ID, sampleFile.Name, bytes.NewReader(content))
		if err != nil {
			log.Printf("Failed to upload sample file %s: %v", sampleFile.Name, err)
			continue
		}

		sample := &domain.SampleFile{
			ProjectID: project.ID,
			FileMetadata: domain.FileMetadata{
				Size:        fileInfo.Size,
				Filename:    fileInfo.Filename,
				ContentType: fileInfo.ContentType,
				Hash:        fileInfo.Hash,
				UploadedAt:  time.Now(),
			},
			FilePath: filePath,
		}

		if err := tx.AddSampleFile(project.ID, sample); err != nil {
			h.storage.DeleteFile(filePath)
			log.Printf("Failed to save sample file info %s: %v", sampleFile.Name, err)
			continue
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		h.renderError(c, "Failed to save project")
		return
	}

	// Return success
	c.HTML(200, "import-success", gin.H{
		"project":     project,
		"sampleCount": len(sampleFiles),
	})
}

func isProjectFile(ext string) bool {
	projectExts := map[string]bool{
		".flp":   true, // FL Studio
		".als":   true, // Ableton
		".logic": true, // Logic
		".ptx":   true, // Pro Tools
		// Add more as needed
	}
	return projectExts[ext]
}

func isAudioFile(ext string) bool {
	audioExts := map[string]bool{
		".wav":  true,
		".mp3":  true,
		".ogg":  true,
		".aiff": true,
		// Add more as needed
	}
	return audioExts[ext]
}

func readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

// renderError renders the error template with given message
func (h *ProjectHandler) renderError(c *gin.Context, message string) {
	c.HTML(http.StatusInternalServerError, "base", gin.H{
		"content": "error",
		"error":   message,
	})
}

func formatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}
