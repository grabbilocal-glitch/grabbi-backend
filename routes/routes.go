package routes

import (
	"grabbi-backend/handlers"
	"grabbi-backend/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func SetupRoutes(r *gin.Engine, db *gorm.DB) {
	// Initialize handlers
	authHandler := &handlers.AuthHandler{DB: db}
	productHandler := &handlers.ProductHandler{DB: db}
	categoryHandler := &handlers.CategoryHandler{DB: db}
	cartHandler := &handlers.CartHandler{DB: db}
	orderHandler := &handlers.OrderHandler{DB: db}
	promotionHandler := &handlers.PromotionHandler{DB: db}

	// Public routes
	api := r.Group("/api")
	{
		// Auth routes
		api.POST("/auth/register", authHandler.Register)
		api.POST("/auth/login", authHandler.Login)

		// Public product routes
		api.GET("/products", productHandler.GetProducts)
		api.GET("/products/:id", productHandler.GetProduct)

		// Public category routes
		api.GET("/categories", categoryHandler.GetCategories)
		api.GET("/categories/:id", categoryHandler.GetCategory)

		// Public promotion routes
		api.GET("/promotions", promotionHandler.GetPromotions)
		api.GET("/promotions/:id", promotionHandler.GetPromotion)
	}

	// Protected routes (require authentication)
	protected := api.Group("")
	protected.Use(middleware.AuthMiddleware())
	{
		// User profile
		protected.GET("/auth/profile", authHandler.GetProfile)

		// Cart routes
		protected.GET("/cart", cartHandler.GetCart)
		protected.POST("/cart", cartHandler.AddToCart)
		protected.PUT("/cart/:id", cartHandler.UpdateCartItem)
		protected.DELETE("/cart/:id", cartHandler.RemoveFromCart)
		protected.DELETE("/cart", cartHandler.ClearCart)

		// Order routes
		protected.POST("/orders", orderHandler.CreateOrder)
		protected.GET("/orders", orderHandler.GetOrders)
		protected.GET("/orders/:id", orderHandler.GetOrder)
	}

	// Admin routes (require admin role)
	admin := api.Group("/admin")
	admin.Use(middleware.AuthMiddleware())
	admin.Use(middleware.AdminMiddleware())
	{
		// Product management
		admin.POST("/products", productHandler.CreateProduct)
		admin.PUT("/products/:id", productHandler.UpdateProduct)
		admin.DELETE("/products/:id", productHandler.DeleteProduct)
		admin.GET("/products", productHandler.GetProductsPaginated)

		// Category management
		admin.POST("/categories", categoryHandler.CreateCategory)
		admin.PUT("/categories/:id", categoryHandler.UpdateCategory)
		admin.DELETE("/categories/:id", categoryHandler.DeleteCategory)

		// Order management
		admin.PUT("/orders/:id/status", orderHandler.UpdateOrderStatus)

		// Promotion management
		admin.POST("/promotions", promotionHandler.CreatePromotion)
		admin.PUT("/promotions/:id", promotionHandler.UpdatePromotion)
		admin.DELETE("/promotions/:id", promotionHandler.DeletePromotion)
	}

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
}
