package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"testing"
	"time"

	"grabbi-backend/middleware"
	"grabbi-backend/models"
	"grabbi-backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var testDB *gorm.DB

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Setenv("JWT_SECRET", "test-secret-key-for-unit-tests")

	var err error
	testDB, err = gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		panic("failed to connect to test database: " + err.Error())
	}
	// Limit to 1 open connection to prevent SQLite concurrent access issues
	// with in-memory databases. This ensures all goroutines (including batch
	// import workers) share the same connection and see the same tables.
	sqlDB, _ := testDB.DB()
	sqlDB.SetMaxOpenConns(1)

	// Create tables using raw SQLite-compatible SQL instead of AutoMigrate,
	// because the GORM model tags use PostgreSQL-specific defaults like gen_random_uuid().
	if err := createSQLiteTables(testDB); err != nil {
		panic("failed to migrate test database: " + err.Error())
	}

	code := m.Run()
	os.Exit(code)
}

// freshDB returns a clean database for each test by deleting all rows.
func freshDB() *gorm.DB {
	// Delete in correct order to respect foreign keys
	testDB.Exec("DELETE FROM order_items")
	testDB.Exec("DELETE FROM orders")
	testDB.Exec("DELETE FROM cart_items")
	testDB.Exec("DELETE FROM franchise_promotions")
	testDB.Exec("DELETE FROM franchise_products")
	testDB.Exec("DELETE FROM franchise_staffs")
	testDB.Exec("DELETE FROM store_hours")
	testDB.Exec("DELETE FROM product_images")
	testDB.Exec("DELETE FROM products")
	testDB.Exec("DELETE FROM subcategories")
	testDB.Exec("DELETE FROM categories")
	testDB.Exec("DELETE FROM franchises")
	testDB.Exec("DELETE FROM promotions")
	testDB.Exec("DELETE FROM password_reset_tokens")
	testDB.Exec("DELETE FROM loyalty_histories")
	testDB.Exec("DELETE FROM refresh_tokens")
	testDB.Exec("DELETE FROM users")
	return testDB
}

// createSQLiteTables creates all tables with SQLite-compatible DDL.
// This avoids GORM AutoMigrate which emits PostgreSQL-specific defaults like gen_random_uuid().
func createSQLiteTables(db *gorm.DB) error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS "users" (
			"id" TEXT PRIMARY KEY,
			"email" TEXT NOT NULL UNIQUE,
			"password" TEXT NOT NULL,
			"name" TEXT,
			"role" TEXT DEFAULT 'customer',
			"franchise_id" TEXT,
			"loyalty_points" INTEGER DEFAULT 0,
			"phone" TEXT,
			"is_blocked" INTEGER DEFAULT 0,
			"created_at" DATETIME,
			"updated_at" DATETIME,
			"deleted_at" DATETIME
		)`,
		`CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON "users"("deleted_at")`,
		`CREATE INDEX IF NOT EXISTS idx_users_franchise_id ON "users"("franchise_id")`,

		`CREATE TABLE IF NOT EXISTS "categories" (
			"id" TEXT PRIMARY KEY,
			"name" TEXT NOT NULL,
			"icon" TEXT,
			"description" TEXT,
			"created_at" DATETIME,
			"updated_at" DATETIME,
			"deleted_at" DATETIME
		)`,
		`CREATE INDEX IF NOT EXISTS idx_categories_deleted_at ON "categories"("deleted_at")`,
		`CREATE INDEX IF NOT EXISTS idx_categories_name ON "categories"("name")`,

		`CREATE TABLE IF NOT EXISTS "subcategories" (
			"id" TEXT PRIMARY KEY,
			"name" TEXT NOT NULL,
			"category_id" TEXT NOT NULL,
			"icon" TEXT,
			"description" TEXT,
			"created_at" DATETIME,
			"updated_at" DATETIME,
			"deleted_at" DATETIME,
			CONSTRAINT fk_subcategories_category FOREIGN KEY ("category_id") REFERENCES "categories"("id")
		)`,
		`CREATE INDEX IF NOT EXISTS idx_subcategories_deleted_at ON "subcategories"("deleted_at")`,
		`CREATE INDEX IF NOT EXISTS idx_subcategories_name ON "subcategories"("name")`,
		`CREATE INDEX IF NOT EXISTS idx_subcategories_category_id ON "subcategories"("category_id")`,

		`CREATE TABLE IF NOT EXISTS "products" (
			"id" TEXT PRIMARY KEY,
			"sku" TEXT NOT NULL UNIQUE,
			"item_name" TEXT NOT NULL,
			"short_description" TEXT,
			"long_description" TEXT,
			"cost_price" REAL NOT NULL,
			"retail_price" REAL NOT NULL,
			"promotion_price" REAL,
			"promotion_start" DATETIME,
			"promotion_end" DATETIME,
			"gross_margin" REAL DEFAULT 0,
			"staff_discount" REAL DEFAULT 0,
			"tax_rate" REAL DEFAULT 0,
			"batch_number" TEXT,
			"barcode" TEXT,
			"stock_quantity" INTEGER DEFAULT 0,
			"reorder_level" INTEGER DEFAULT 0,
			"shelf_location" TEXT,
			"weight_volume" REAL DEFAULT 0,
			"unit_of_measure" TEXT,
			"expiry_date" DATETIME,
			"category_id" TEXT NOT NULL,
			"subcategory_id" TEXT,
			"brand" TEXT,
			"supplier" TEXT,
			"country_of_origin" TEXT,
			"is_gluten_free" INTEGER DEFAULT 0,
			"is_vegetarian" INTEGER DEFAULT 0,
			"is_vegan" INTEGER DEFAULT 0,
			"is_age_restricted" INTEGER DEFAULT 0,
			"minimum_age" INTEGER,
			"allergen_info" TEXT,
			"storage_type" TEXT,
			"is_own_brand" INTEGER DEFAULT 0,
			"online_visible" INTEGER DEFAULT 1,
			"status" TEXT DEFAULT 'active',
			"notes" TEXT,
			"pack_size" TEXT,
			"created_at" DATETIME,
			"updated_at" DATETIME,
			"deleted_at" DATETIME,
			CONSTRAINT fk_products_category FOREIGN KEY ("category_id") REFERENCES "categories"("id")
		)`,
		`CREATE INDEX IF NOT EXISTS idx_products_deleted_at ON "products"("deleted_at")`,
		`CREATE INDEX IF NOT EXISTS idx_products_item_name ON "products"("item_name")`,
		`CREATE INDEX IF NOT EXISTS idx_products_category_id ON "products"("category_id")`,
		`CREATE INDEX IF NOT EXISTS idx_products_subcategory_id ON "products"("subcategory_id")`,
		`CREATE INDEX IF NOT EXISTS idx_products_status ON "products"("status")`,
		`CREATE INDEX IF NOT EXISTS idx_products_stock_quantity ON "products"("stock_quantity")`,
		`CREATE INDEX IF NOT EXISTS idx_products_batch_number ON "products"("batch_number")`,
		`CREATE INDEX IF NOT EXISTS idx_products_shelf_location ON "products"("shelf_location")`,
		`CREATE INDEX IF NOT EXISTS idx_products_expiry_date ON "products"("expiry_date")`,
		`CREATE INDEX IF NOT EXISTS idx_products_brand ON "products"("brand")`,
		`CREATE INDEX IF NOT EXISTS idx_products_supplier ON "products"("supplier")`,
		`CREATE INDEX IF NOT EXISTS idx_products_is_gluten_free ON "products"("is_gluten_free")`,
		`CREATE INDEX IF NOT EXISTS idx_products_is_vegetarian ON "products"("is_vegetarian")`,
		`CREATE INDEX IF NOT EXISTS idx_products_is_vegan ON "products"("is_vegan")`,
		`CREATE INDEX IF NOT EXISTS idx_products_is_age_restricted ON "products"("is_age_restricted")`,
		`CREATE INDEX IF NOT EXISTS idx_products_storage_type ON "products"("storage_type")`,
		`CREATE INDEX IF NOT EXISTS idx_products_is_own_brand ON "products"("is_own_brand")`,
		`CREATE INDEX IF NOT EXISTS idx_products_online_visible ON "products"("online_visible")`,

		`CREATE TABLE IF NOT EXISTS "product_images" (
			"id" TEXT PRIMARY KEY,
			"product_id" TEXT NOT NULL,
			"image_url" TEXT NOT NULL,
			"is_primary" INTEGER DEFAULT 0,
			"created_at" DATETIME,
			"updated_at" DATETIME,
			"deleted_at" DATETIME,
			CONSTRAINT fk_product_images_product FOREIGN KEY ("product_id") REFERENCES "products"("id")
		)`,
		`CREATE INDEX IF NOT EXISTS idx_product_images_deleted_at ON "product_images"("deleted_at")`,
		`CREATE INDEX IF NOT EXISTS idx_product_images_product_id ON "product_images"("product_id")`,

		`CREATE TABLE IF NOT EXISTS "promotions" (
			"id" TEXT PRIMARY KEY,
			"title" TEXT NOT NULL,
			"description" TEXT,
			"image" TEXT,
			"product_url" TEXT,
			"is_active" INTEGER DEFAULT 1,
			"start_date" DATETIME,
			"end_date" DATETIME,
			"created_at" DATETIME,
			"updated_at" DATETIME,
			"deleted_at" DATETIME
		)`,
		`CREATE INDEX IF NOT EXISTS idx_promotions_deleted_at ON "promotions"("deleted_at")`,

		`CREATE TABLE IF NOT EXISTS "franchises" (
			"id" TEXT PRIMARY KEY,
			"name" TEXT NOT NULL,
			"slug" TEXT NOT NULL UNIQUE,
			"owner_id" TEXT NOT NULL,
			"address" TEXT,
			"city" TEXT,
			"post_code" TEXT,
			"latitude" REAL NOT NULL,
			"longitude" REAL NOT NULL,
			"delivery_radius" REAL DEFAULT 5,
			"delivery_fee" REAL DEFAULT 4.99,
			"free_delivery_min" REAL DEFAULT 50,
			"phone" TEXT,
			"email" TEXT,
			"is_active" INTEGER DEFAULT 1,
			"created_at" DATETIME,
			"updated_at" DATETIME,
			"deleted_at" DATETIME,
			CONSTRAINT fk_franchises_owner FOREIGN KEY ("owner_id") REFERENCES "users"("id")
		)`,
		`CREATE INDEX IF NOT EXISTS idx_franchises_deleted_at ON "franchises"("deleted_at")`,

		`CREATE TABLE IF NOT EXISTS "store_hours" (
			"id" TEXT PRIMARY KEY,
			"franchise_id" TEXT NOT NULL,
			"day_of_week" INTEGER NOT NULL,
			"open_time" TEXT NOT NULL DEFAULT '09:00',
			"close_time" TEXT NOT NULL DEFAULT '21:00',
			"is_closed" INTEGER DEFAULT 0,
			"created_at" DATETIME,
			"updated_at" DATETIME,
			CONSTRAINT fk_store_hours_franchise FOREIGN KEY ("franchise_id") REFERENCES "franchises"("id")
		)`,
		`CREATE INDEX IF NOT EXISTS idx_store_hours_franchise_id ON "store_hours"("franchise_id")`,

		`CREATE TABLE IF NOT EXISTS "franchise_products" (
			"id" TEXT PRIMARY KEY,
			"franchise_id" TEXT NOT NULL,
			"product_id" TEXT NOT NULL,
			"retail_price_override" REAL,
			"promotion_price_override" REAL,
			"promotion_start_override" DATETIME,
			"promotion_end_override" DATETIME,
			"stock_quantity" INTEGER DEFAULT 0,
			"reorder_level" INTEGER DEFAULT 5,
			"shelf_location" TEXT,
			"is_available" INTEGER DEFAULT 1,
			"created_at" DATETIME,
			"updated_at" DATETIME,
			CONSTRAINT fk_franchise_products_franchise FOREIGN KEY ("franchise_id") REFERENCES "franchises"("id"),
			CONSTRAINT fk_franchise_products_product FOREIGN KEY ("product_id") REFERENCES "products"("id")
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_franchise_product ON "franchise_products"("franchise_id","product_id")`,

		`CREATE TABLE IF NOT EXISTS "franchise_staffs" (
			"id" TEXT PRIMARY KEY,
			"franchise_id" TEXT NOT NULL,
			"user_id" TEXT NOT NULL UNIQUE,
			"role" TEXT NOT NULL DEFAULT 'staff',
			"created_at" DATETIME,
			"updated_at" DATETIME,
			CONSTRAINT fk_franchise_staffs_franchise FOREIGN KEY ("franchise_id") REFERENCES "franchises"("id"),
			CONSTRAINT fk_franchise_staffs_user FOREIGN KEY ("user_id") REFERENCES "users"("id")
		)`,
		`CREATE INDEX IF NOT EXISTS idx_franchise_staffs_franchise_id ON "franchise_staffs"("franchise_id")`,

		`CREATE TABLE IF NOT EXISTS "franchise_promotions" (
			"id" TEXT PRIMARY KEY,
			"franchise_id" TEXT NOT NULL,
			"title" TEXT NOT NULL,
			"description" TEXT,
			"image" TEXT,
			"product_url" TEXT,
			"is_active" INTEGER DEFAULT 1,
			"start_date" DATETIME,
			"end_date" DATETIME,
			"created_at" DATETIME,
			"updated_at" DATETIME,
			"deleted_at" DATETIME,
			CONSTRAINT fk_franchise_promotions_franchise FOREIGN KEY ("franchise_id") REFERENCES "franchises"("id")
		)`,
		`CREATE INDEX IF NOT EXISTS idx_franchise_promotions_deleted_at ON "franchise_promotions"("deleted_at")`,
		`CREATE INDEX IF NOT EXISTS idx_franchise_promotions_franchise_id ON "franchise_promotions"("franchise_id")`,

		`CREATE TABLE IF NOT EXISTS "cart_items" (
			"id" TEXT PRIMARY KEY,
			"user_id" TEXT NOT NULL,
			"product_id" TEXT NOT NULL,
			"franchise_id" TEXT,
			"quantity" INTEGER DEFAULT 1,
			"created_at" DATETIME,
			"updated_at" DATETIME,
			"deleted_at" DATETIME,
			CONSTRAINT fk_cart_items_user FOREIGN KEY ("user_id") REFERENCES "users"("id"),
			CONSTRAINT fk_cart_items_product FOREIGN KEY ("product_id") REFERENCES "products"("id")
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_cart_user_product ON "cart_items"("user_id","product_id")`,
		`CREATE INDEX IF NOT EXISTS idx_cart_items_deleted_at ON "cart_items"("deleted_at")`,
		`CREATE INDEX IF NOT EXISTS idx_cart_items_franchise_id ON "cart_items"("franchise_id")`,

		`CREATE TABLE IF NOT EXISTS "orders" (
			"id" TEXT PRIMARY KEY,
			"user_id" TEXT NOT NULL,
			"franchise_id" TEXT,
			"order_number" TEXT NOT NULL UNIQUE,
			"status" TEXT DEFAULT 'pending',
			"subtotal" REAL NOT NULL,
			"delivery_fee" REAL DEFAULT 0,
			"total" REAL NOT NULL,
			"delivery_address" TEXT,
			"payment_method" TEXT,
			"points_earned" INTEGER DEFAULT 0,
			"customer_lat" REAL,
			"customer_lng" REAL,
			"created_at" DATETIME,
			"updated_at" DATETIME,
			"deleted_at" DATETIME,
			CONSTRAINT fk_orders_user FOREIGN KEY ("user_id") REFERENCES "users"("id")
		)`,
		`CREATE INDEX IF NOT EXISTS idx_orders_deleted_at ON "orders"("deleted_at")`,
		`CREATE INDEX IF NOT EXISTS idx_orders_user_id ON "orders"("user_id")`,
		`CREATE INDEX IF NOT EXISTS idx_orders_franchise_id ON "orders"("franchise_id")`,

		`CREATE TABLE IF NOT EXISTS "order_items" (
			"id" TEXT PRIMARY KEY,
			"order_id" TEXT NOT NULL,
			"product_id" TEXT NOT NULL,
			"image_url" TEXT,
			"quantity" INTEGER NOT NULL,
			"price" REAL NOT NULL,
			"created_at" DATETIME,
			"updated_at" DATETIME,
			CONSTRAINT fk_order_items_order FOREIGN KEY ("order_id") REFERENCES "orders"("id"),
			CONSTRAINT fk_order_items_product FOREIGN KEY ("product_id") REFERENCES "products"("id")
		)`,
		`CREATE INDEX IF NOT EXISTS idx_order_items_order_id ON "order_items"("order_id")`,
		`CREATE INDEX IF NOT EXISTS idx_order_items_product_id ON "order_items"("product_id")`,

		`CREATE TABLE IF NOT EXISTS "password_reset_tokens" (
			"id" TEXT PRIMARY KEY,
			"user_id" TEXT NOT NULL,
			"token" TEXT NOT NULL UNIQUE,
			"expires_at" DATETIME NOT NULL,
			"used_at" DATETIME,
			"created_at" DATETIME,
			CONSTRAINT fk_password_reset_tokens_user FOREIGN KEY ("user_id") REFERENCES "users"("id")
		)`,
		`CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user_id ON "password_reset_tokens"("user_id")`,

		`CREATE TABLE IF NOT EXISTS "loyalty_histories" (
			"id" TEXT PRIMARY KEY,
			"user_id" TEXT NOT NULL,
			"points" INTEGER NOT NULL,
			"type" TEXT NOT NULL,
			"description" TEXT,
			"order_id" TEXT,
			"created_at" DATETIME,
			CONSTRAINT fk_loyalty_histories_user FOREIGN KEY ("user_id") REFERENCES "users"("id")
		)`,
		`CREATE INDEX IF NOT EXISTS idx_loyalty_histories_user_id ON "loyalty_histories"("user_id")`,

		`CREATE TABLE IF NOT EXISTS "refresh_tokens" (
			"id" TEXT PRIMARY KEY,
			"user_id" TEXT NOT NULL,
			"token" TEXT NOT NULL UNIQUE,
			"expires_at" DATETIME NOT NULL,
			"revoked_at" DATETIME,
			"created_at" DATETIME,
			CONSTRAINT fk_refresh_tokens_user FOREIGN KEY ("user_id") REFERENCES "users"("id")
		)`,
		`CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON "refresh_tokens"("user_id")`,
	}

	for _, sql := range tables {
		if err := db.Exec(sql).Error; err != nil {
			return err
		}
	}
	return nil
}

// seedTestUser creates a user with the given role and returns it along with a valid JWT token.
func seedTestUser(db *gorm.DB, email, role string, franchiseID *uuid.UUID) (models.User, string) {
	hashed, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	user := models.User{
		ID:          uuid.New(),
		Email:       email,
		Password:    string(hashed),
		Name:        "Test User",
		Role:        role,
		FranchiseID: franchiseID,
	}
	db.Create(&user)

	token, _ := utils.GenerateToken(user.ID, user.Email, user.Role, franchiseID)
	return user, token
}

// seedCategory creates a test category.
func seedCategory(db *gorm.DB, name string) models.Category {
	cat := models.Category{
		ID:   uuid.New(),
		Name: name,
	}
	db.Create(&cat)
	return cat
}

// seedProduct creates a test product.
func seedProduct(db *gorm.DB, name string, categoryID uuid.UUID, price float64) models.Product {
	prod := models.Product{
		ID:            uuid.New(),
		SKU:           "SKU-" + uuid.New().String()[:8],
		ItemName:      name,
		RetailPrice:   price,
		CostPrice:     price * 0.5,
		CategoryID:    categoryID,
		StockQuantity: 100,
		Status:        "active",
		OnlineVisible: true,
		Barcode:       "BAR-" + uuid.New().String()[:8],
	}
	db.Create(&prod)
	return prod
}

// seedPromotion creates a test promotion.
// After creation, explicitly updates is_active to handle the case where GORM skips
// the zero-value (false) and the DB default (true/1) takes effect.
func seedPromotion(db *gorm.DB, title string, active bool) models.Promotion {
	promo := models.Promotion{
		ID:       uuid.New(),
		Title:    title,
		IsActive: active,
	}
	db.Create(&promo)
	// Explicitly update is_active to ensure false values are persisted,
	// since GORM may skip zero-value bools during Create.
	db.Model(&promo).Update("is_active", active)
	return promo
}

// seedFranchise creates a test franchise.
func seedFranchise(db *gorm.DB, name string, ownerID uuid.UUID) models.Franchise {
	franchise := models.Franchise{
		ID:              uuid.New(),
		Name:            name,
		Slug:            "test-franchise-" + uuid.New().String()[:8],
		OwnerID:         ownerID,
		Latitude:        51.5074,
		Longitude:       -0.1278,
		DeliveryRadius:  5.0,
		DeliveryFee:     4.99,
		FreeDeliveryMin: 50.0,
		IsActive:        true,
	}
	db.Create(&franchise)
	return franchise
}

// seedFranchiseOwnerWithToken creates a franchise_owner user with the given franchise's ID set,
// and returns the user and a valid JWT token.
func seedFranchiseOwnerWithToken(db *gorm.DB, franchise models.Franchise) (models.User, string) {
	franchiseID := franchise.ID
	return seedTestUser(db, "owner-"+uuid.New().String()[:8]+"@test.com", "franchise_owner", &franchiseID)
}

// seedStoreHours creates 7 store hours records (Mon-Sun) for the given franchise.
func seedStoreHours(db *gorm.DB, franchiseID uuid.UUID) []models.StoreHours {
	days := []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
	hours := make([]models.StoreHours, len(days))
	for i := range days {
		h := models.StoreHours{
			ID:          uuid.New(),
			FranchiseID: franchiseID,
			DayOfWeek:   i,
			OpenTime:    "09:00",
			CloseTime:   "21:00",
			IsClosed:    false,
		}
		db.Create(&h)
		hours[i] = h
	}
	return hours
}

// seedFranchiseProduct creates a FranchiseProduct linking a franchise to a product.
func seedFranchiseProduct(db *gorm.DB, franchiseID, productID uuid.UUID) models.FranchiseProduct {
	fp := models.FranchiseProduct{
		ID:            uuid.New(),
		FranchiseID:   franchiseID,
		ProductID:     productID,
		StockQuantity: 50,
		ReorderLevel:  5,
		IsAvailable:   true,
	}
	db.Create(&fp)
	return fp
}

// seedFranchiseStaff creates a FranchiseStaff record.
func seedFranchiseStaff(db *gorm.DB, franchiseID, userID uuid.UUID, role string) models.FranchiseStaff {
	fs := models.FranchiseStaff{
		ID:          uuid.New(),
		FranchiseID: franchiseID,
		UserID:      userID,
		Role:        role,
	}
	db.Create(&fs)
	return fs
}

// seedFranchisePromotion creates a FranchisePromotion.
func seedFranchisePromotion(db *gorm.DB, franchiseID uuid.UUID, title string) models.FranchisePromotion {
	fp := models.FranchisePromotion{
		ID:          uuid.New(),
		FranchiseID: franchiseID,
		Title:       title,
		IsActive:    true,
	}
	db.Create(&fp)
	return fp
}

// seedOrder creates an Order with one OrderItem.
func seedOrder(db *gorm.DB, userID, franchiseID, productID uuid.UUID) models.Order {
	orderID := uuid.New()
	fID := franchiseID
	order := models.Order{
		ID:          orderID,
		UserID:      userID,
		FranchiseID: &fID,
		OrderNumber: "ORD" + time.Now().Format("20060102150405") + orderID.String()[:8],
		Status:      models.OrderStatusPending,
		Subtotal:    10.00,
		DeliveryFee: 4.99,
		Total:       14.99,
		Items: []models.OrderItem{
			{
				ID:        uuid.New(),
				OrderID:   orderID,
				ProductID: productID,
				Quantity:  1,
				Price:     10.00,
			},
		},
	}
	db.Create(&order)
	return order
}

// seedSubcategory creates a Subcategory.
func seedSubcategory(db *gorm.DB, name string, categoryID uuid.UUID) models.Subcategory {
	sub := models.Subcategory{
		ID:         uuid.New(),
		Name:       name,
		CategoryID: categoryID,
	}
	db.Create(&sub)
	return sub
}

// ==================== Router Setup Helpers ====================

// setupAuthRouter sets up routes for auth handler tests.
func setupAuthRouter(db *gorm.DB) *gin.Engine {
	r := gin.New()
	authHandler := &AuthHandler{DB: db}

	api := r.Group("/api")
	api.POST("/auth/register", authHandler.Register)
	api.POST("/auth/login", authHandler.Login)

	protected := api.Group("")
	protected.Use(middleware.AuthMiddleware())
	protected.GET("/auth/profile", authHandler.GetProfile)

	return r
}

// setupProductRouter sets up routes for product handler tests.
func setupProductRouter(db *gorm.DB) *gin.Engine {
	r := gin.New()
	productHandler := &ProductHandler{DB: db, Storage: newMockStorage()}

	api := r.Group("/api")

	// Public routes
	api.GET("/products", productHandler.GetProducts)
	api.GET("/products/:id", productHandler.GetProduct)

	// Admin routes
	admin := api.Group("/admin")
	admin.Use(middleware.AuthMiddleware())
	admin.Use(middleware.AdminMiddleware())
	admin.POST("/products", productHandler.CreateProduct)
	admin.PUT("/products/:id", productHandler.UpdateProduct)
	admin.DELETE("/products/:id", productHandler.DeleteProduct)
	admin.GET("/products", productHandler.GetProductsPaginated)
	admin.GET("/products/export", productHandler.GetProductsExport)
	admin.POST("/products/batch", productHandler.BatchImportProducts)
	admin.GET("/products/batch/:id", productHandler.GetBatchJobStatus)

	return r
}

// setupOrderRouter sets up routes for order handler tests.
func setupOrderRouter(db *gorm.DB) *gin.Engine {
	r := gin.New()
	orderHandler := &OrderHandler{DB: db}

	api := r.Group("/api")

	protected := api.Group("")
	protected.Use(middleware.AuthMiddleware())
	protected.POST("/orders", orderHandler.CreateOrder)
	protected.GET("/orders", orderHandler.GetOrders)
	protected.GET("/orders/:id", orderHandler.GetOrder)

	admin := api.Group("/admin")
	admin.Use(middleware.AuthMiddleware())
	admin.Use(middleware.AdminMiddleware())
	admin.PUT("/orders/:id/status", orderHandler.UpdateOrderStatus)

	return r
}

// setupCartRouter sets up routes for cart handler tests.
func setupCartRouter(db *gorm.DB) *gin.Engine {
	r := gin.New()
	cartHandler := &CartHandler{DB: db}

	api := r.Group("/api")
	protected := api.Group("")
	protected.Use(middleware.AuthMiddleware())
	protected.GET("/cart", cartHandler.GetCart)
	protected.POST("/cart", cartHandler.AddToCart)
	protected.PUT("/cart/:id", cartHandler.UpdateCartItem)
	protected.DELETE("/cart/:id", cartHandler.RemoveFromCart)
	protected.DELETE("/cart", cartHandler.ClearCart)

	return r
}

// setupFranchiseRouter sets up routes for franchise handler tests.
func setupFranchiseRouter(db *gorm.DB) *gin.Engine {
	r := gin.New()
	franchiseHandler := &FranchiseHandler{DB: db}

	api := r.Group("/api")

	// Public routes
	api.GET("/franchises/nearest", franchiseHandler.GetNearestFranchise)
	api.GET("/franchises/:id", franchiseHandler.GetFranchise)
	api.GET("/franchises/:id/products", franchiseHandler.GetFranchiseProducts)
	api.GET("/franchises/:id/promotions", franchiseHandler.GetFranchisePromotions)

	// Admin routes
	admin := api.Group("/admin")
	admin.Use(middleware.AuthMiddleware())
	admin.Use(middleware.AdminMiddleware())
	admin.GET("/franchises", franchiseHandler.ListFranchises)
	admin.POST("/franchises", franchiseHandler.CreateFranchise)
	admin.PUT("/franchises/:id", franchiseHandler.UpdateFranchise)
	admin.DELETE("/franchises/:id", franchiseHandler.DeleteFranchise)
	admin.GET("/franchises/:id/orders", franchiseHandler.GetFranchiseOrders)

	return r
}

// setupCategoryRouter sets up routes for category handler tests.
func setupCategoryRouter(db *gorm.DB) *gin.Engine {
	r := gin.New()
	categoryHandler := &CategoryHandler{DB: db}

	api := r.Group("/api")
	api.GET("/categories", categoryHandler.GetCategories)
	api.GET("/categories/:id", categoryHandler.GetCategory)

	admin := api.Group("/admin")
	admin.Use(middleware.AuthMiddleware())
	admin.Use(middleware.AdminMiddleware())
	admin.POST("/categories", categoryHandler.CreateCategory)
	admin.PUT("/categories/:id", categoryHandler.UpdateCategory)
	admin.DELETE("/categories/:id", categoryHandler.DeleteCategory)

	return r
}

// setupPromotionRouter sets up routes for promotion handler tests.
func setupPromotionRouter(db *gorm.DB) *gin.Engine {
	r := gin.New()
	promotionHandler := &PromotionHandler{DB: db, Storage: newMockStorage()}

	api := r.Group("/api")

	// Public routes
	api.GET("/promotions", promotionHandler.GetPromotions)
	api.GET("/promotions/:id", promotionHandler.GetPromotion)

	// Admin routes
	admin := api.Group("/admin")
	admin.Use(middleware.AuthMiddleware())
	admin.Use(middleware.AdminMiddleware())
	admin.POST("/promotions", promotionHandler.CreatePromotion)
	admin.PUT("/promotions/:id", promotionHandler.UpdatePromotion)
	admin.DELETE("/promotions/:id", promotionHandler.DeletePromotion)

	return r
}

// setupSubcategoryRouter sets up routes for subcategory handler tests.
func setupSubcategoryRouter(db *gorm.DB) *gin.Engine {
	r := gin.New()
	subcategoryHandler := &SubcategoryHandler{DB: db}

	api := r.Group("/api")

	// Public routes
	api.GET("/subcategories", subcategoryHandler.GetSubcategories)

	// Admin routes
	admin := api.Group("/admin")
	admin.Use(middleware.AuthMiddleware())
	admin.Use(middleware.AdminMiddleware())
	admin.POST("/subcategories", subcategoryHandler.CreateSubcategory)
	admin.PUT("/subcategories/:id", subcategoryHandler.UpdateSubcategory)
	admin.DELETE("/subcategories/:id", subcategoryHandler.DeleteSubcategory)

	return r
}

// setupFranchisePortalRouter sets up all franchise portal routes for tests.
func setupFranchisePortalRouter(db *gorm.DB) *gin.Engine {
	r := gin.New()
	franchiseHandler := &FranchiseHandler{DB: db}

	api := r.Group("/api")
	franchise := api.Group("/franchise")
	franchise.Use(middleware.AuthMiddleware())
	franchise.Use(middleware.FranchiseMiddleware())

	franchise.GET("/me", franchiseHandler.GetMyFranchise)
	franchise.PUT("/me", franchiseHandler.UpdateMyFranchise)

	franchise.GET("/products", franchiseHandler.GetMyProducts)
	franchise.PUT("/products/:id/stock", franchiseHandler.UpdateProductStock)
	franchise.PUT("/products/:id/pricing", franchiseHandler.UpdateProductPricing)

	franchise.GET("/orders", franchiseHandler.GetMyOrders)
	franchise.PUT("/orders/:id/status", franchiseHandler.UpdateOrderStatus)

	franchise.GET("/staff", franchiseHandler.GetMyStaff)
	franchise.POST("/staff", franchiseHandler.InviteStaff)
	franchise.DELETE("/staff/:id", franchiseHandler.RemoveStaff)

	franchise.GET("/hours", franchiseHandler.GetStoreHours)
	franchise.PUT("/hours", franchiseHandler.UpdateStoreHours)

	franchise.GET("/promotions", franchiseHandler.GetMyPromotions)
	franchise.POST("/promotions", franchiseHandler.CreatePromotion)
	franchise.PUT("/promotions/:id", franchiseHandler.UpdatePromotion)
	franchise.DELETE("/promotions/:id", franchiseHandler.DeletePromotion)

	franchise.GET("/dashboard", franchiseHandler.GetDashboard)

	return r
}

// ==================== Request Helpers ====================

// jsonRequest creates an HTTP request with JSON body.
func jsonRequest(method, url string, body interface{}) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, url, &buf)
	req.Header.Set("Content-Type", "application/json")
	return req
}

// authRequest creates an HTTP request with JSON body and Authorization header.
func authRequest(method, url string, body interface{}, token string) *http.Request {
	req := jsonRequest(method, url, body)
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

// multipartRequest creates a multipart form request with the given fields and file uploads.
// fields is a map of form field names to values.
// files is a map of form field names to filenames (dummy file data is used).
// token is the JWT token for the Authorization header (pass "" to skip).
func multipartRequest(method, url string, fields map[string]string, files map[string]string, token string) *http.Request {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Write form fields
	for key, val := range fields {
		_ = writer.WriteField(key, val)
	}

	// Write file parts
	for fieldName, filename := range files {
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fieldName, filename))
		h.Set("Content-Type", "image/jpeg")

		part, err := writer.CreatePart(h)
		if err != nil {
			panic("failed to create multipart file part: " + err.Error())
		}
		// Write dummy image data
		part.Write([]byte("fake image data"))
	}

	writer.Close()

	req := httptest.NewRequest(method, url, &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

// ==================== Response Helpers ====================

// parseResponse reads the response body into a map.
func parseResponse(w *httptest.ResponseRecorder) map[string]interface{} {
	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	return result
}

// parseResponseArray reads the response body into a slice of maps.
func parseResponseArray(w *httptest.ResponseRecorder) []interface{} {
	var result []interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	return result
}

// Ensure time import is used
var _ = time.Now
