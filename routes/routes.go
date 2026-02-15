package routes

import (
	"net/http"
	"time"

	"grabbi-backend/firebase"
	"grabbi-backend/handlers"
	"grabbi-backend/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func SetupRoutes(r *gin.Engine, db *gorm.DB, storage firebase.StorageClient) {
	// Initialize handlers
	authHandler := &handlers.AuthHandler{DB: db}
	productHandler := &handlers.ProductHandler{DB: db, Storage: storage}
	categoryHandler := &handlers.CategoryHandler{DB: db}
	subcategoryHandler := &handlers.SubcategoryHandler{DB: db}
	cartHandler := &handlers.CartHandler{DB: db}
	orderHandler := &handlers.OrderHandler{DB: db}
	promotionHandler := &handlers.PromotionHandler{DB: db, Storage: storage}
	franchiseHandler := &handlers.FranchiseHandler{DB: db}

	// Rate limiters
	authRateLimiter := middleware.NewRateLimiter(5, 1*time.Minute)
	passwordResetRateLimiter := middleware.NewRateLimiter(3, 1*time.Minute)
	orderRateLimiter := middleware.NewRateLimiter(10, 1*time.Minute)
	cartRateLimiter := middleware.NewRateLimiter(30, 1*time.Minute)

	// Public routes
	api := r.Group("/api")
	{
		// Auth routes (rate limited: 5 requests/minute per IP)
		authGroup := api.Group("/auth")
		authGroup.Use(authRateLimiter.Middleware())
		{
			authGroup.POST("/register", authHandler.Register)
			authGroup.POST("/login", authHandler.Login)
		}

		// Password reset routes (rate limited: 3 requests/minute per IP)
		passwordResetGroup := api.Group("/auth")
		passwordResetGroup.Use(passwordResetRateLimiter.Middleware())
		{
			passwordResetGroup.POST("/forgot-password", authHandler.ForgotPassword)
			passwordResetGroup.POST("/reset-password", authHandler.ResetPassword)
		}

		// Refresh token (public, no auth required)
		api.POST("/auth/refresh", authHandler.RefreshTokenHandler)

		// Public product routes
		api.GET("/products", productHandler.GetProducts)
		api.GET("/products/:id", productHandler.GetProduct)

		// Public category routes
		api.GET("/categories", categoryHandler.GetCategories)
		api.GET("/categories/:id", categoryHandler.GetCategory)

		// Public subcategory routes
		api.GET("/subcategories", subcategoryHandler.GetSubcategories)

		// Public promotion routes
		api.GET("/promotions", promotionHandler.GetPromotions)
		api.GET("/promotions/:id", promotionHandler.GetPromotion)

		// Public franchise routes
		api.GET("/franchises/nearest", franchiseHandler.GetNearestFranchise)
		api.GET("/franchises/:id", franchiseHandler.GetFranchise)
		api.GET("/franchises/:id/products", franchiseHandler.GetFranchiseProducts)
		api.GET("/franchises/:id/promotions", franchiseHandler.GetFranchisePromotions)
	}

	// Protected routes (require authentication)
	protected := api.Group("")
	protected.Use(middleware.AuthMiddleware())
	{
		// User profile
		protected.GET("/auth/profile", authHandler.GetProfile)
		protected.PUT("/auth/profile", authHandler.UpdateProfile)
		protected.PUT("/auth/password", authHandler.ChangePassword)

		// Loyalty
		protected.POST("/auth/redeem-points", authHandler.RedeemPoints)
		protected.GET("/auth/loyalty-history", authHandler.GetLoyaltyHistory)

		// Cart routes
		protected.GET("/cart", cartHandler.GetCart)

		// Cart write routes (rate limited: 30 requests/minute per IP)
		cartWrite := protected.Group("")
		cartWrite.Use(cartRateLimiter.Middleware())
		{
			cartWrite.POST("/cart", cartHandler.AddToCart)
			cartWrite.PUT("/cart/:id", cartHandler.UpdateCartItem)
		}
		protected.DELETE("/cart/:id", cartHandler.RemoveFromCart)
		protected.DELETE("/cart", cartHandler.ClearCart)

		// Order routes
		orderWrite := protected.Group("")
		orderWrite.Use(orderRateLimiter.Middleware())
		{
			orderWrite.POST("/orders", orderHandler.CreateOrder)
		}
		protected.GET("/orders", orderHandler.GetOrders)
		protected.GET("/orders/:id", orderHandler.GetOrder)
		protected.GET("/orders/transitions", orderHandler.GetOrderTransitions)
	}

	// Franchise portal routes (require franchise role)
	// Read-only routes: accessible by both franchise_owner and franchise_staff
	franchise := api.Group("/franchise")
	franchise.Use(middleware.AuthMiddleware())
	franchise.Use(middleware.FranchiseMiddleware())
	{
		franchise.GET("/me", franchiseHandler.GetMyFranchise)
		franchise.GET("/products", franchiseHandler.GetMyProducts)
		franchise.GET("/orders", franchiseHandler.GetMyOrders)
		franchise.GET("/staff", franchiseHandler.GetMyStaff)
		franchise.GET("/hours", franchiseHandler.GetStoreHours)
		franchise.GET("/promotions", franchiseHandler.GetMyPromotions)
		franchise.GET("/dashboard", franchiseHandler.GetDashboard)

		// Write operations available to all franchise roles
		franchise.PUT("/products/:id/stock", franchiseHandler.UpdateProductStock)
		franchise.PUT("/orders/:id/status", franchiseHandler.UpdateOrderStatus)
	}

	// Franchise owner-only write routes
	franchiseOwner := api.Group("/franchise")
	franchiseOwner.Use(middleware.AuthMiddleware())
	franchiseOwner.Use(middleware.FranchiseMiddleware())
	franchiseOwner.Use(middleware.FranchiseOwnerMiddleware())
	{
		franchiseOwner.PUT("/me", franchiseHandler.UpdateMyFranchise)
		franchiseOwner.POST("/products", franchiseHandler.CreateProduct)
		franchiseOwner.PUT("/products/:id/pricing", franchiseHandler.UpdateProductPricing)

		franchiseOwner.POST("/staff", franchiseHandler.InviteStaff)
		franchiseOwner.DELETE("/staff/:id", franchiseHandler.RemoveStaff)

		franchiseOwner.PUT("/hours", franchiseHandler.UpdateStoreHours)

		franchiseOwner.POST("/promotions", franchiseHandler.CreatePromotion)
		franchiseOwner.PUT("/promotions/:id", franchiseHandler.UpdatePromotion)
		franchiseOwner.DELETE("/promotions/:id", franchiseHandler.DeletePromotion)
	}

	// Admin routes (require admin role)
	admin := api.Group("/admin")
	admin.Use(middleware.AuthMiddleware())
	admin.Use(middleware.AdminMiddleware())
	{
		// Product management
		admin.POST("/products", productHandler.CreateProduct)
		admin.POST("/products/batch", productHandler.BatchImportProducts)
		admin.GET("/products/batch/:id", productHandler.GetBatchJobStatus)
		admin.PUT("/products/:id", productHandler.UpdateProduct)
		admin.DELETE("/products/:id", productHandler.DeleteProduct)
		admin.GET("/products", productHandler.GetProductsPaginated)
		admin.GET("/products/export", productHandler.GetProductsExport)
		admin.GET("/products/:id/franchises", productHandler.GetProductFranchises)

		// Category management
		admin.POST("/categories", categoryHandler.CreateCategory)
		admin.PUT("/categories/:id", categoryHandler.UpdateCategory)
		admin.DELETE("/categories/:id", categoryHandler.DeleteCategory)

		// Subcategory management
		admin.POST("/subcategories", subcategoryHandler.CreateSubcategory)
		admin.PUT("/subcategories/:id", subcategoryHandler.UpdateSubcategory)
		admin.DELETE("/subcategories/:id", subcategoryHandler.DeleteSubcategory)

		// Admin dashboard
		admin.GET("/dashboard", orderHandler.GetAdminDashboard)

		// Order management
		admin.PUT("/orders/:id/status", orderHandler.UpdateOrderStatus)

		// Promotion management
		admin.GET("/promotions", promotionHandler.GetAllPromotions)
		admin.POST("/promotions", promotionHandler.CreatePromotion)
		admin.PUT("/promotions/:id", promotionHandler.UpdatePromotion)
		admin.DELETE("/promotions/:id", promotionHandler.DeletePromotion)

		// Franchise management (super admin)
		admin.GET("/franchises", franchiseHandler.ListFranchises)
		admin.POST("/franchises", franchiseHandler.CreateFranchise)
		admin.PUT("/franchises/:id", franchiseHandler.UpdateFranchise)
		admin.DELETE("/franchises/:id", franchiseHandler.DeleteFranchise)
		admin.GET("/franchises/:id/orders", franchiseHandler.GetFranchiseOrders)

		// User management
		admin.GET("/users", authHandler.ListUsers)
		admin.GET("/users/:id", authHandler.GetUser)
		admin.PUT("/users/:id", authHandler.UpdateUser)
	}

	// Health check with database verification
	r.GET("/health", func(c *gin.Context) {
		sqlDB, err := db.DB()
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":   "degraded",
				"database": "disconnected",
			})
			return
		}
		if err := sqlDB.Ping(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":   "degraded",
				"database": "disconnected",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status":   "ok",
			"database": "connected",
		})
	})
}
