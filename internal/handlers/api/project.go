package api

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"dawhub/internal/domain"
)

// ProjectHandler handles HTTP requests for project operations
type ProjectHandler struct {
	repo    domain.ProjectRepository
	storage domain.StorageService
}

// NewProjectHandler creates a new project handler with the given repository and storage service
func NewProjectHandler(repo domain.ProjectRepository, storage domain.StorageService) *ProjectHandler {
	return &ProjectHandler{
		repo:    repo,
		storage: storage,
	}
}

// Create handles POST /projects to create a new project
func (h *ProjectHandler) Create(c *gin.Context) {
	var project domain.Project
	if err := c.ShouldBindJSON(&project); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.repo.Create(&project); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create project"})
		return
	}

	c.JSON(http.StatusCreated, project)
}

// List handles GET /projects to retrieve all projects
func (h *ProjectHandler) List(c *gin.Context) {
	projects, err := h.repo.FindAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch projects"})
		return
	}

	c.JSON(http.StatusOK, projects)
}

// Get handles GET /projects/:id to retrieve a specific project
func (h *ProjectHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	project, err := h.repo.FindByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	c.JSON(http.StatusOK, project)
}

// Update handles PUT /projects/:id to modify an existing project
func (h *ProjectHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	project, err := h.repo.FindByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	if err := c.ShouldBindJSON(project); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.repo.Update(project); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update project"})
		return
	}

	c.JSON(http.StatusOK, project)
}

// Delete handles DELETE /projects/:id to remove a project
func (h *ProjectHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	if err := h.repo.Delete(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete project"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// Upload handles POST /projects/:id/upload for file uploads
func (h *ProjectHandler) Upload(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	project, err := h.repo.FindByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file"})
		return
	}
	defer file.Close()

	// Start transaction
	tx, err := h.repo.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	// Upload file and get metadata
	fileInfo, filePath, err := h.storage.UploadFile(project.ID, header.Filename, file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload file"})
		return
	}

	// Create SampleFile record
	sampleFile := &domain.SampleFile{
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

	// Add sample file to project
	if err := tx.AddSampleFile(project.ID, sampleFile); err != nil {
		// Cleanup uploaded file on error
		h.storage.DeleteFile(filePath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file information"})
		return
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		h.storage.DeleteFile(filePath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update project"})
		return
	}

	// Return success response with HTMX
	if c.GetHeader("HX-Request") == "true" {
		c.HTML(http.StatusOK, "upload-response", gin.H{
			"success": true,
			"file": gin.H{
				"name": fileInfo.Filename,
				"size": domain.FormatFileSize(fileInfo.Size),
				"type": fileInfo.ContentType,
				"path": filePath,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"file": gin.H{
			"name": fileInfo.Filename,
			"size": fileInfo.Size,
			"type": fileInfo.ContentType,
			"path": filePath,
		},
	})
}

// Download handles GET /projects/:id/download to serve project files
func (h *ProjectHandler) Download(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	project, err := h.repo.FindByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	fileType := c.Query("type")
	fileId := c.Query("fileId")

	var filePath string
	var fileName string

	switch fileType {
	case "main":
		if project.MainFile == nil || project.MainFile.FilePath == "" {
			c.JSON(http.StatusNotFound, gin.H{"error": "No main file associated with this project"})
			return
		}
		filePath = project.MainFile.FilePath
		fileName = project.MainFile.Filename
	case "sample":
		if fileId == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "File ID is required for sample files"})
			return
		}

		sampleFileId, err := strconv.ParseUint(fileId, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file ID"})
			return
		}

		var sampleFile *domain.SampleFile
		for _, sf := range project.SampleFiles {
			if sf.ID == uint(sampleFileId) {
				sampleFile = &sf
				break
			}
		}

		if sampleFile == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Sample file not found"})
			return
		}
		filePath = sampleFile.FilePath
		fileName = sampleFile.Filename
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file type"})
		return
	}

	obj, fileInfo, err := h.storage.GetFile(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get file"})
		return
	}
	defer obj.Close()

	// Set headers for download
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileName))
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Length", fmt.Sprintf("%d", fileInfo.Size))

	// Stream the file to the client
	if _, err := io.Copy(c.Writer, obj); err != nil {
		log.Printf("Error streaming file: %v", err)
		return
	}
}
