package server

import (
	"fmt"
	"html/template"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"

	"dawhub/internal/config"
	"dawhub/internal/email"
	"dawhub/internal/handlers/api"
	"dawhub/internal/handlers/web"
	"dawhub/internal/middleware"
	"dawhub/internal/repository"
	"dawhub/internal/storage"
)

type Server struct {
	config     *config.Config
	router     *gin.Engine
	templates  *template.Template
	projectAPI *api.ProjectHandler
	projectWeb *web.ProjectHandler
	authAPI    *api.AuthHandler
	authWeb    *web.AuthHandler
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

	// Initialize email
	emailService := email.NewResendService(cfg.Email)

	// Initialize repositories
	projectRepo := repository.NewProjectRepository(db)
	userRepo := repository.NewUserRepository(db)

	// Initialize handlers
	projectAPI := api.NewProjectHandler(projectRepo, store)
	projectWeb := web.NewProjectHandler(projectRepo, store)
	authAPI := api.NewAuthHandler(userRepo)
	authWeb := web.NewAuthHandler(userRepo, projectRepo, emailService)

	// Initialize router
	router := gin.New()

	// Add essential middlewares
	router.Use(gin.Recovery())                            // Recover from panics
	router.Use(middleware.NewIPRateLimiter().RateLimit()) // Rate limiting
	router.Use(middleware.BlockSuspiciousRequests())      // Block suspicious requests
	router.Use(middleware.LogSuspiciousRequests())        // Log suspiciouis requests

	// Add session middleware
	cookieStore := cookie.NewStore([]byte(cfg.Server.SessionSecret))
	router.Use(sessions.Sessions("dawhub_session", cookieStore))

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
		authAPI:    authAPI,
		authWeb:    authWeb,
	}, nil
}

func (s *Server) setupRoutes() {
	// Serve static files
	s.router.Static("/static", "./static")

	// Set HTML templates
	s.router.SetHTMLTemplate(s.templates)

	// Auth routes
	s.router.GET("/login", s.authWeb.LoginPage)
	s.router.POST("/login", s.authWeb.Login)
	// s.router.GET("/register", s.authWeb.RegisterPage)
	// s.router.POST("/register", s.authWeb.Register)
	s.router.POST("/logout", s.authWeb.Logout)
	s.router.GET("/health", s.authWeb.Health)
	s.router.GET("/", s.authWeb.LandingPage)

	// Beta Signup Route
	s.router.POST("/beta-signup", s.authWeb.BetaSignup)

	// Protected web routes
	web := s.router.Group("/")
	web.Use(middleware.WebAuthMiddleware())
	{
		web.GET("/dashboard", s.projectWeb.Home)
		web.GET("/projects", s.projectWeb.List)
		web.GET("/projects/new", s.projectWeb.New)
		web.POST("/projects/create", s.projectWeb.Create)
		web.GET("/projects/:id", s.projectWeb.Show)
		web.GET("/projects/:id/edit", s.projectWeb.Edit)
		web.POST("/projects/:id/update", s.projectWeb.Update)
		web.POST("/projects/:id/delete", s.projectWeb.Delete)
		web.GET("/projects/import", s.projectWeb.Import)
		web.POST("/projects/import", s.projectWeb.HandleImport)
		web.GET("/settings", s.authWeb.SettingsPage)
		web.POST("/settings/profile", s.authWeb.UpdateProfile)
		web.POST("/settings/password", s.authWeb.UpdatePassword)
		web.POST("/settings/delete-account", s.authWeb.DeleteAccount)
		web.GET("/beta-users", s.authWeb.BetaUsersPage)
	}

	// API routes
	api := s.router.Group("/api/v1")
	{
		// Public API routes
		api.POST("/login", s.authAPI.Login)
		api.POST("/register", s.authAPI.Register)

		// Protected API routes
		protected := api.Group("")
		protected.Use(middleware.APIAuthMiddleware())
		{
			protected.GET("/projects", s.projectAPI.List)
			protected.POST("/projects", s.projectAPI.Create)
			protected.GET("/projects/:id", s.projectAPI.Get)
			protected.PUT("/projects/:id", s.projectAPI.Update)
			protected.DELETE("/projects/:id", s.projectAPI.Delete)
		}

		// Separate download route with dual auth
		api.GET("/projects/:id/download", middleware.DualAuthMiddleware(), s.projectAPI.Download)
		api.POST("/projects/:id/upload", middleware.DualAuthMiddleware(), s.projectAPI.Upload)
	}
}

func (s *Server) Start() error {
	s.setupRoutes()
	return s.router.Run(":" + s.config.Server.Port)
}
