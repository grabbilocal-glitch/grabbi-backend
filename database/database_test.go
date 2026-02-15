package database

import (
	"os"
	"testing"

	"grabbi-backend/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

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
		`CREATE TABLE IF NOT EXISTS "categories" (
			"id" TEXT PRIMARY KEY,
			"name" TEXT NOT NULL,
			"icon" TEXT,
			"description" TEXT,
			"created_at" DATETIME,
			"updated_at" DATETIME,
			"deleted_at" DATETIME
		)`,
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
	}

	for _, sql := range tables {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestCreateDefaultAdminNew(t *testing.T) {
	db := setupTestDB(t)
	os.Setenv("ADMIN_EMAIL", "testadmin@test.com")
	os.Setenv("ADMIN_PASSWORD", "testpassword123")
	defer os.Unsetenv("ADMIN_EMAIL")
	defer os.Unsetenv("ADMIN_PASSWORD")

	err := CreateDefaultAdmin(db)
	if err != nil {
		t.Fatal(err)
	}

	var user models.User
	if err := db.Where("email = ?", "testadmin@test.com").First(&user).Error; err != nil {
		t.Fatal("admin user not created")
	}
	if user.Role != "admin" {
		t.Errorf("expected role 'admin', got '%s'", user.Role)
	}
}

func TestCreateDefaultAdminAlreadyExists(t *testing.T) {
	db := setupTestDB(t)
	os.Setenv("ADMIN_EMAIL", "existing@test.com")
	os.Setenv("ADMIN_PASSWORD", "password123")
	defer os.Unsetenv("ADMIN_EMAIL")
	defer os.Unsetenv("ADMIN_PASSWORD")

	// Create admin first time
	err := CreateDefaultAdmin(db)
	if err != nil {
		t.Fatal(err)
	}

	// Second call should skip (no error)
	err = CreateDefaultAdmin(db)
	if err != nil {
		t.Fatal(err)
	}

	var count int64
	db.Model(&models.User{}).Where("email = ?", "existing@test.com").Count(&count)
	if count != 1 {
		t.Errorf("expected exactly 1 admin, got %d", count)
	}
}

func TestCreateDefaultAdminRandomPassword(t *testing.T) {
	db := setupTestDB(t)
	os.Setenv("ADMIN_EMAIL", "random@test.com")
	os.Unsetenv("ADMIN_PASSWORD")
	defer os.Unsetenv("ADMIN_EMAIL")

	err := CreateDefaultAdmin(db)
	if err != nil {
		t.Fatal(err)
	}

	var user models.User
	if err := db.Where("email = ?", "random@test.com").First(&user).Error; err != nil {
		t.Fatal("admin not created with random password")
	}
}

func TestCreateDefaultFranchiseNew(t *testing.T) {
	db := setupTestDB(t)
	os.Setenv("ADMIN_EMAIL", "admin@franchise-test.com")
	os.Setenv("ADMIN_PASSWORD", "password123")
	defer os.Unsetenv("ADMIN_EMAIL")
	defer os.Unsetenv("ADMIN_PASSWORD")

	// Create admin first
	CreateDefaultAdmin(db)

	err := CreateDefaultFranchise(db)
	if err != nil {
		t.Fatal(err)
	}

	var franchise models.Franchise
	if err := db.First(&franchise).Error; err != nil {
		t.Fatal("franchise not created")
	}
	if franchise.Name != "Grabbi Main Store" {
		t.Errorf("expected 'Grabbi Main Store', got '%s'", franchise.Name)
	}

	// Check store hours were created
	var hoursCount int64
	db.Model(&models.StoreHours{}).Where("franchise_id = ?", franchise.ID).Count(&hoursCount)
	if hoursCount != 7 {
		t.Errorf("expected 7 store hours, got %d", hoursCount)
	}
}

func TestCreateDefaultFranchiseAlreadyExists(t *testing.T) {
	db := setupTestDB(t)
	os.Setenv("ADMIN_EMAIL", "admin@skip-test.com")
	os.Setenv("ADMIN_PASSWORD", "password123")
	defer os.Unsetenv("ADMIN_EMAIL")
	defer os.Unsetenv("ADMIN_PASSWORD")

	CreateDefaultAdmin(db)
	CreateDefaultFranchise(db)

	// Second call should skip
	err := CreateDefaultFranchise(db)
	if err != nil {
		t.Fatal(err)
	}

	var count int64
	db.Model(&models.Franchise{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 franchise, got %d", count)
	}
}

func TestCreateDefaultFranchiseNoAdmin(t *testing.T) {
	db := setupTestDB(t)

	// No admin user exists - should return nil gracefully
	err := CreateDefaultFranchise(db)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}

	var count int64
	db.Model(&models.Franchise{}).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 franchises, got %d", count)
	}
}
