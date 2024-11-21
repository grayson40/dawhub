package server

import (
	"fmt"
	"html/template"

	"github.com/gin-gonic/gin"

	"dawhub/internal/config"
	"dawhub/internal/handlers/api"
	"dawhub/internal/handlers/web"
	"dawhub/internal/repository"
	"dawhub/internal/storage"
)

type Server struct {
	config     *config.Config
	router     *gin.Engine
	templates  *template.Template
	projectAPI *api.ProjectHandler
	projectWeb *web.ProjectHandler
}

func New(cfg *config.Config) (*Server, error) {
	// Initialize database
	db, err := repository.NewDB(cfg.DB)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Initialize storage
	store, err := storage.NewMinioStorage(cfg.Minio)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	// Initialize repositories
	projectRepo := repository.NewProjectRepository(db)

	// Initialize handlers
	projectAPI := api.NewProjectHandler(projectRepo, store)
	projectWeb := web.NewProjectHandler(projectRepo, store)

	// Initialize router
	router := gin.Default()

	// Parse templates
	templates, err := template.ParseGlob("templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	return &Server{
		config:     cfg,
		router:     router,
		templates:  templates,
		projectAPI: projectAPI,
		projectWeb: projectWeb,
	}, nil
}

func (s *Server) setupRoutes() {
	// Serve static files
	s.router.Static("/static", "./static")

	// Set HTML templates
	s.router.SetHTMLTemplate(s.templates)

	// Web routes
	s.router.GET("/", s.projectWeb.Home)
	s.router.GET("/projects", s.projectWeb.List)
	s.router.GET("/projects/new", s.projectWeb.New)
	s.router.POST("/projects/create", s.projectWeb.Create)
	s.router.GET("/projects/:id", s.projectWeb.Show)
	s.router.GET("/projects/:id/edit", s.projectWeb.Edit)
	s.router.POST("/projects/:id/update", s.projectWeb.Update)
	s.router.POST("/projects/:id/delete", s.projectWeb.Delete)
	s.router.GET("/projects/import", s.projectWeb.Import)
	s.router.POST("/projects/import", s.projectWeb.HandleImport)

	// API routes
	api := s.router.Group("/api/v1")
	{
		api.GET("/projects", s.projectAPI.List)
		api.POST("/projects", s.projectAPI.Create)
		api.GET("/projects/:id", s.projectAPI.Get)
		api.PUT("/projects/:id", s.projectAPI.Update)
		api.DELETE("/projects/:id", s.projectAPI.Delete)
		api.POST("/projects/:id/upload", s.projectAPI.Upload)
		api.GET("/projects/:id/download", s.projectAPI.Download)
	}
}

func (s *Server) Start() error {
	s.setupRoutes()
	return s.router.Run(":" + s.config.Server.Port)
}
