package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"grabbi-backend/config"
	"grabbi-backend/database"
	"grabbi-backend/firebase"
	"grabbi-backend/routes"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	// Load environment variables
	if err := config.LoadEnv(); err != nil {
		log.Fatal("Error loading .env file:", err)
	}

	// Validate critical environment variables
	if err := config.ValidateEnv(); err != nil {
		log.Fatal("Environment validation failed: ", err)
	}

	// Initialize database
	db, err := database.Connect()
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Run migrations
	if err := database.Migrate(db); err != nil {
		log.Fatal("Failed to run migrations:", err)
	}

	// Create default admin user if not exists
	if err := database.CreateDefaultAdmin(db); err != nil {
		log.Printf("Warning: Could not create default admin: %v", err)
	}

	// Create default franchise if not exists
	if err := database.CreateDefaultFranchise(db); err != nil {
		log.Printf("Warning: Could not create default franchise: %v", err)
	}

	//firebase init
	firebase.Init()
	storageClient := firebase.NewStorageClient()

	// Setup Gin router
	r := gin.Default()

	// Limit multipart form memory to 10MB
	r.MaxMultipartMemory = 10 << 20

	// CORS configuration - filter out empty strings from AllowOrigins
	origins := []string{os.Getenv("FRONTEND_URL"), os.Getenv("ADMIN_URL"), os.Getenv("FRANCHISE_URL")}
	var filteredOrigins []string
	for _, o := range origins {
		if o != "" {
			filteredOrigins = append(filteredOrigins, o)
		}
	}
	if len(filteredOrigins) == 0 {
		filteredOrigins = []string{"http://localhost:3000"}
		log.Println("WARNING: No CORS origins configured, defaulting to http://localhost:3000")
	}

	r.Use(cors.New(cors.Config{
		AllowOrigins:     filteredOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// Setup routes
	routes.SetupRoutes(r, db, storageClient)

	// Start server with graceful shutdown
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	// Run server in a goroutine
	go func() {
		log.Printf("Server starting on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Give outstanding requests 30 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	// Close database connection
	sqlDB, err := db.DB()
	if err == nil {
		if err := sqlDB.Close(); err != nil {
			log.Printf("Error closing database connection: %v", err)
		} else {
			log.Println("Database connection closed")
		}
	}

	log.Println("Server exited gracefully")
}
