package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
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
			"id" TEXT PRIMARY KEY, "email" TEXT NOT NULL UNIQUE, "password" TEXT NOT NULL,
			"name" TEXT, "role" TEXT DEFAULT 'customer', "franchise_id" TEXT,
			"loyalty_points" INTEGER DEFAULT 0, "phone" TEXT, "is_blocked" INTEGER DEFAULT 0,
			"created_at" DATETIME, "updated_at" DATETIME, "deleted_at" DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS "categories" (
			"id" TEXT PRIMARY KEY, "name" TEXT NOT NULL, "icon" TEXT, "description" TEXT,
			"created_at" DATETIME, "updated_at" DATETIME, "deleted_at" DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS "subcategories" (
			"id" TEXT PRIMARY KEY, "name" TEXT NOT NULL, "category_id" TEXT NOT NULL,
			"icon" TEXT, "description" TEXT, "created_at" DATETIME, "updated_at" DATETIME, "deleted_at" DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS "products" (
			"id" TEXT PRIMARY KEY, "sku" TEXT NOT NULL UNIQUE, "item_name" TEXT NOT NULL,
			"short_description" TEXT, "long_description" TEXT, "cost_price" REAL NOT NULL,
			"retail_price" REAL NOT NULL, "promotion_price" REAL, "promotion_start" DATETIME,
			"promotion_end" DATETIME, "gross_margin" REAL DEFAULT 0, "staff_discount" REAL DEFAULT 0,
			"tax_rate" REAL DEFAULT 0, "batch_number" TEXT, "barcode" TEXT, "stock_quantity" INTEGER DEFAULT 0,
			"reorder_level" INTEGER DEFAULT 0, "shelf_location" TEXT, "weight_volume" REAL DEFAULT 0,
			"unit_of_measure" TEXT, "expiry_date" DATETIME, "category_id" TEXT NOT NULL,
			"subcategory_id" TEXT, "brand" TEXT, "supplier" TEXT, "country_of_origin" TEXT,
			"is_gluten_free" INTEGER DEFAULT 0, "is_vegetarian" INTEGER DEFAULT 0, "is_vegan" INTEGER DEFAULT 0,
			"is_age_restricted" INTEGER DEFAULT 0, "minimum_age" INTEGER, "allergen_info" TEXT,
			"storage_type" TEXT, "is_own_brand" INTEGER DEFAULT 0, "online_visible" INTEGER DEFAULT 1,
			"status" TEXT DEFAULT 'active', "notes" TEXT, "pack_size" TEXT,
			"created_at" DATETIME, "updated_at" DATETIME, "deleted_at" DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS "product_images" (
			"id" TEXT PRIMARY KEY, "product_id" TEXT NOT NULL, "image_url" TEXT NOT NULL,
			"is_primary" INTEGER DEFAULT 0, "created_at" DATETIME, "updated_at" DATETIME, "deleted_at" DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS "promotions" (
			"id" TEXT PRIMARY KEY, "title" TEXT NOT NULL, "description" TEXT, "image" TEXT,
			"product_url" TEXT, "is_active" INTEGER DEFAULT 1,
			"start_date" DATETIME, "end_date" DATETIME,
			"created_at" DATETIME, "updated_at" DATETIME, "deleted_at" DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS "franchises" (
			"id" TEXT PRIMARY KEY, "name" TEXT NOT NULL, "slug" TEXT NOT NULL UNIQUE,
			"owner_id" TEXT NOT NULL, "address" TEXT, "city" TEXT, "post_code" TEXT,
			"latitude" REAL NOT NULL, "longitude" REAL NOT NULL, "delivery_radius" REAL DEFAULT 5,
			"delivery_fee" REAL DEFAULT 4.99, "free_delivery_min" REAL DEFAULT 50,
			"phone" TEXT, "email" TEXT, "is_active" INTEGER DEFAULT 1,
			"created_at" DATETIME, "updated_at" DATETIME, "deleted_at" DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS "store_hours" (
			"id" TEXT PRIMARY KEY, "franchise_id" TEXT NOT NULL, "day_of_week" INTEGER NOT NULL,
			"open_time" TEXT NOT NULL DEFAULT '09:00', "close_time" TEXT NOT NULL DEFAULT '21:00',
			"is_closed" INTEGER DEFAULT 0, "created_at" DATETIME, "updated_at" DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS "franchise_products" (
			"id" TEXT PRIMARY KEY, "franchise_id" TEXT NOT NULL, "product_id" TEXT NOT NULL,
			"retail_price_override" REAL, "promotion_price_override" REAL,
			"promotion_start_override" DATETIME, "promotion_end_override" DATETIME,
			"stock_quantity" INTEGER DEFAULT 0, "reorder_level" INTEGER DEFAULT 5,
			"shelf_location" TEXT, "is_available" INTEGER DEFAULT 1,
			"created_at" DATETIME, "updated_at" DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS "franchise_staffs" (
			"id" TEXT PRIMARY KEY, "franchise_id" TEXT NOT NULL, "user_id" TEXT NOT NULL UNIQUE,
			"role" TEXT NOT NULL DEFAULT 'staff', "created_at" DATETIME, "updated_at" DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS "franchise_promotions" (
			"id" TEXT PRIMARY KEY, "franchise_id" TEXT NOT NULL, "title" TEXT NOT NULL,
			"description" TEXT, "image" TEXT, "product_url" TEXT, "is_active" INTEGER DEFAULT 1,
			"start_date" DATETIME, "end_date" DATETIME,
			"created_at" DATETIME, "updated_at" DATETIME, "deleted_at" DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS "cart_items" (
			"id" TEXT PRIMARY KEY, "user_id" TEXT NOT NULL, "product_id" TEXT NOT NULL,
			"franchise_id" TEXT, "quantity" INTEGER DEFAULT 1,
			"created_at" DATETIME, "updated_at" DATETIME, "deleted_at" DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS "orders" (
			"id" TEXT PRIMARY KEY, "user_id" TEXT NOT NULL, "franchise_id" TEXT,
			"order_number" TEXT NOT NULL UNIQUE, "status" TEXT DEFAULT 'pending',
			"subtotal" REAL NOT NULL, "delivery_fee" REAL DEFAULT 0, "total" REAL NOT NULL,
			"delivery_address" TEXT, "payment_method" TEXT, "points_earned" INTEGER DEFAULT 0,
			"customer_lat" REAL, "customer_lng" REAL,
			"created_at" DATETIME, "updated_at" DATETIME, "deleted_at" DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS "order_items" (
			"id" TEXT PRIMARY KEY, "order_id" TEXT NOT NULL, "product_id" TEXT NOT NULL,
			"image_url" TEXT, "quantity" INTEGER NOT NULL, "price" REAL NOT NULL,
			"created_at" DATETIME, "updated_at" DATETIME
		)`,
	}

	for _, sql := range tables {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

// ==================== BeforeCreate Hook Tests ====================

func TestUserBeforeCreateGeneratesUUID(t *testing.T) {
	db := setupTestDB(t)
	user := User{Email: "test@test.com", Password: "hash", Name: "Test"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if user.ID == uuid.Nil {
		t.Error("UUID should have been generated")
	}
}

func TestUserBeforeCreatePreservesUUID(t *testing.T) {
	db := setupTestDB(t)
	existingID := uuid.New()
	user := User{ID: existingID, Email: "preserve@test.com", Password: "hash", Name: "Test"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if user.ID != existingID {
		t.Error("UUID should have been preserved")
	}
}

func TestCategoryBeforeCreate(t *testing.T) {
	db := setupTestDB(t)
	cat := Category{Name: "Test"}
	db.Create(&cat)
	if cat.ID == uuid.Nil {
		t.Error("UUID should have been generated")
	}
}

func TestCategoryBeforeCreatePreserves(t *testing.T) {
	db := setupTestDB(t)
	id := uuid.New()
	cat := Category{ID: id, Name: "Preserved"}
	db.Create(&cat)
	if cat.ID != id {
		t.Error("UUID should have been preserved")
	}
}

func TestProductBeforeCreate(t *testing.T) {
	db := setupTestDB(t)
	cat := Category{ID: uuid.New(), Name: "Cat"}
	db.Create(&cat)
	prod := Product{SKU: "TEST-SKU", ItemName: "Test", CostPrice: 1, RetailPrice: 2, CategoryID: cat.ID}
	db.Create(&prod)
	if prod.ID == uuid.Nil {
		t.Error("UUID should have been generated")
	}
}

func TestSubcategoryBeforeCreate(t *testing.T) {
	db := setupTestDB(t)
	cat := Category{ID: uuid.New(), Name: "Cat"}
	db.Create(&cat)
	sub := Subcategory{Name: "Sub", CategoryID: cat.ID}
	db.Create(&sub)
	if sub.ID == uuid.Nil {
		t.Error("UUID should have been generated")
	}
}

func TestPromotionBeforeCreate(t *testing.T) {
	db := setupTestDB(t)
	promo := Promotion{Title: "Test"}
	db.Create(&promo)
	if promo.ID == uuid.Nil {
		t.Error("UUID should have been generated")
	}
}

func TestFranchiseBeforeCreate(t *testing.T) {
	db := setupTestDB(t)
	owner := User{ID: uuid.New(), Email: "owner@test.com", Password: "hash"}
	db.Create(&owner)
	f := Franchise{Name: "Test", Slug: "test", OwnerID: owner.ID, Latitude: 51, Longitude: -0.1}
	db.Create(&f)
	if f.ID == uuid.Nil {
		t.Error("UUID should have been generated")
	}
}

func TestStoreHoursBeforeCreate(t *testing.T) {
	db := setupTestDB(t)
	owner := User{ID: uuid.New(), Email: "sh-owner@test.com", Password: "hash"}
	db.Create(&owner)
	f := Franchise{ID: uuid.New(), Name: "F", Slug: "f-slug", OwnerID: owner.ID, Latitude: 51, Longitude: -0.1}
	db.Create(&f)
	sh := StoreHours{FranchiseID: f.ID, DayOfWeek: 0, OpenTime: "09:00", CloseTime: "21:00"}
	db.Create(&sh)
	if sh.ID == uuid.Nil {
		t.Error("UUID should have been generated")
	}
}

func TestProductImageBeforeCreate(t *testing.T) {
	db := setupTestDB(t)
	cat := Category{ID: uuid.New(), Name: "Cat"}
	db.Create(&cat)
	prod := Product{ID: uuid.New(), SKU: "IMG-SKU", ItemName: "P", CostPrice: 1, RetailPrice: 2, CategoryID: cat.ID}
	db.Create(&prod)
	img := ProductImage{ProductID: prod.ID, ImageURL: "http://test.com/img.jpg"}
	db.Create(&img)
	if img.ID == uuid.Nil {
		t.Error("UUID should have been generated")
	}
}

func TestFranchiseProductBeforeCreate(t *testing.T) {
	db := setupTestDB(t)
	owner := User{ID: uuid.New(), Email: "fp-owner@test.com", Password: "hash"}
	db.Create(&owner)
	f := Franchise{ID: uuid.New(), Name: "F", Slug: "fp-slug", OwnerID: owner.ID, Latitude: 51, Longitude: -0.1}
	db.Create(&f)
	cat := Category{ID: uuid.New(), Name: "Cat"}
	db.Create(&cat)
	prod := Product{ID: uuid.New(), SKU: "FP-SKU", ItemName: "P", CostPrice: 1, RetailPrice: 2, CategoryID: cat.ID}
	db.Create(&prod)
	fp := FranchiseProduct{FranchiseID: f.ID, ProductID: prod.ID, StockQuantity: 10}
	db.Create(&fp)
	if fp.ID == uuid.Nil {
		t.Error("UUID should have been generated")
	}
}

func TestFranchiseStaffBeforeCreate(t *testing.T) {
	db := setupTestDB(t)
	owner := User{ID: uuid.New(), Email: "fs-owner@test.com", Password: "hash"}
	db.Create(&owner)
	f := Franchise{ID: uuid.New(), Name: "F", Slug: "fs-slug", OwnerID: owner.ID, Latitude: 51, Longitude: -0.1}
	db.Create(&f)
	staff := User{ID: uuid.New(), Email: "staff@test.com", Password: "hash"}
	db.Create(&staff)
	fs := FranchiseStaff{FranchiseID: f.ID, UserID: staff.ID, Role: "staff"}
	db.Create(&fs)
	if fs.ID == uuid.Nil {
		t.Error("UUID should have been generated")
	}
}

func TestFranchisePromotionBeforeCreate(t *testing.T) {
	db := setupTestDB(t)
	owner := User{ID: uuid.New(), Email: "fpr-owner@test.com", Password: "hash"}
	db.Create(&owner)
	f := Franchise{ID: uuid.New(), Name: "F", Slug: "fpr-slug", OwnerID: owner.ID, Latitude: 51, Longitude: -0.1}
	db.Create(&f)
	fp := FranchisePromotion{FranchiseID: f.ID, Title: "Promo"}
	db.Create(&fp)
	if fp.ID == uuid.Nil {
		t.Error("UUID should have been generated")
	}
}

func TestCartItemBeforeCreate(t *testing.T) {
	db := setupTestDB(t)
	user := User{ID: uuid.New(), Email: "cart@test.com", Password: "hash"}
	db.Create(&user)
	cat := Category{ID: uuid.New(), Name: "Cat"}
	db.Create(&cat)
	prod := Product{ID: uuid.New(), SKU: "CART-SKU", ItemName: "P", CostPrice: 1, RetailPrice: 2, CategoryID: cat.ID}
	db.Create(&prod)
	ci := CartItem{UserID: user.ID, ProductID: prod.ID, Quantity: 1}
	db.Create(&ci)
	if ci.ID == uuid.Nil {
		t.Error("UUID should have been generated")
	}
}

func TestOrderBeforeCreate(t *testing.T) {
	db := setupTestDB(t)
	user := User{ID: uuid.New(), Email: "order@test.com", Password: "hash"}
	db.Create(&user)
	order := Order{UserID: user.ID, Subtotal: 10, Total: 10}
	db.Create(&order)
	if order.ID == uuid.Nil {
		t.Error("UUID should have been generated")
	}
	if order.OrderNumber == "" {
		t.Error("OrderNumber should have been generated")
	}
}

// ==================== Product Method Tests ====================

func TestIsPromotionActiveNoPrice(t *testing.T) {
	p := Product{RetailPrice: 10.0}
	if p.IsPromotionActive() {
		t.Error("should be false when no promotion price")
	}
}

func TestIsPromotionActiveNoDates(t *testing.T) {
	price := 5.0
	p := Product{RetailPrice: 10.0, PromotionPrice: &price}
	if p.IsPromotionActive() {
		t.Error("should be false when no dates set")
	}
}

func TestIsPromotionActiveFutureStart(t *testing.T) {
	price := 5.0
	future := time.Now().Add(24 * time.Hour)
	p := Product{RetailPrice: 10.0, PromotionPrice: &price, PromotionStart: &future}
	if p.IsPromotionActive() {
		t.Error("should be false when start is in the future")
	}
}

func TestIsPromotionActivePastEnd(t *testing.T) {
	price := 5.0
	past := time.Now().Add(-24 * time.Hour)
	p := Product{RetailPrice: 10.0, PromotionPrice: &price, PromotionEnd: &past}
	if p.IsPromotionActive() {
		t.Error("should be false when end is in the past")
	}
}

func TestIsPromotionActiveWithinRange(t *testing.T) {
	price := 5.0
	start := time.Now().Add(-24 * time.Hour)
	end := time.Now().Add(24 * time.Hour)
	p := Product{RetailPrice: 10.0, PromotionPrice: &price, PromotionStart: &start, PromotionEnd: &end}
	if !p.IsPromotionActive() {
		t.Error("should be true when within range")
	}
}

func TestIsPromotionActiveOnlyStartSet(t *testing.T) {
	price := 5.0
	start := time.Now().Add(-24 * time.Hour)
	p := Product{RetailPrice: 10.0, PromotionPrice: &price, PromotionStart: &start}
	if !p.IsPromotionActive() {
		t.Error("should be true when only start is set and is in the past")
	}
}

func TestIsPromotionActiveOnlyEndSet(t *testing.T) {
	price := 5.0
	end := time.Now().Add(24 * time.Hour)
	p := Product{RetailPrice: 10.0, PromotionPrice: &price, PromotionEnd: &end}
	if !p.IsPromotionActive() {
		t.Error("should be true when only end is set and is in the future")
	}
}

func TestGetCurrentPriceRetail(t *testing.T) {
	p := Product{RetailPrice: 10.0}
	if p.GetCurrentPrice() != 10.0 {
		t.Errorf("expected 10.0, got %f", p.GetCurrentPrice())
	}
}

func TestGetCurrentPricePromo(t *testing.T) {
	price := 5.0
	start := time.Now().Add(-24 * time.Hour)
	end := time.Now().Add(24 * time.Hour)
	p := Product{RetailPrice: 10.0, PromotionPrice: &price, PromotionStart: &start, PromotionEnd: &end}
	if p.GetCurrentPrice() != 5.0 {
		t.Errorf("expected 5.0, got %f", p.GetCurrentPrice())
	}
}
