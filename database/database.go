package database

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"time"

	"grabbi-backend/models"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Connect() (*gorm.DB, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "host=localhost user=postgres password=postgres dbname=grabbi_store port=5432 sslmode=disable"
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// Get underlying SQL DB for connection pool configuration
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal("Failed to get underlying sql.DB:", err)
	}

	// Optimize connection pool for concurrent batch import operations
	sqlDB.SetMaxOpenConns(25)                  // Increase from default (2) to handle concurrent workers
	sqlDB.SetMaxIdleConns(10)                  // Keep idle connections ready
	sqlDB.SetConnMaxLifetime(5 * time.Minute)  // Reuse connections for 5 minutes
	sqlDB.SetConnMaxIdleTime(30 * time.Second) // Close idle connections after 30s

	log.Printf("Database connection pool configured: MaxOpen=25, MaxIdle=10")

	return db, nil
}

func Migrate(db *gorm.DB) error {
	// Ensure PostgreSQL has gen_random_uuid() available (pgcrypto extension).
	if err := db.Exec(`CREATE EXTENSION IF NOT EXISTS pgcrypto;`).Error; err != nil {
		return fmt.Errorf("failed to enable pgcrypto extension: %w", err)
	}

	if err := db.AutoMigrate(
		&models.User{},
		&models.Category{},
		&models.Subcategory{},
		&models.Product{},
		&models.ProductImage{},
		&models.CartItem{},
		&models.Order{},
		&models.OrderItem{},
		&models.Promotion{},
		&models.Franchise{},
		&models.StoreHours{},
		&models.FranchiseProduct{},
		&models.FranchiseStaff{},
		&models.FranchisePromotion{},
		&models.PasswordResetToken{},
		&models.LoyaltyHistory{},
		&models.RefreshToken{},
	); err != nil {
		return err
	}

	// AutoMigrate will NOT fix an existing incorrect primary key. If the DB was created
	// with a wrong PK on order_items (e.g. order_id), inserts will fail with:
	// "duplicate key value violates unique constraint order_items_pkey".
	if err := repairOrderItemsPrimaryKey(db); err != nil {
		return err
	}

	// Create SKU sequence and function for auto-generating product SKUs
	if err := db.Exec(`CREATE SEQUENCE IF NOT EXISTS sku_seq START 1;`).Error; err != nil {
		return fmt.Errorf("failed to create sku_seq sequence: %w", err)
	}
	if err := db.Exec(`
		CREATE OR REPLACE FUNCTION generate_next_sku() RETURNS TEXT AS $$
			SELECT 'GRB-' || LPAD(nextval('sku_seq')::TEXT, 6, '0');
		$$ LANGUAGE SQL;
	`).Error; err != nil {
		return fmt.Errorf("failed to create generate_next_sku function: %w", err)
	}

	return nil
}

func CreateDefaultAdmin(db *gorm.DB) error {
	adminEmail := os.Getenv("ADMIN_EMAIL")
	adminPassword := os.Getenv("ADMIN_PASSWORD")

	if adminEmail == "" {
		adminEmail = "admin@grabbi.com"
		log.Println("WARNING: ADMIN_EMAIL not set, defaulting to admin@grabbi.com")
	}
	if adminPassword == "" {
		// Generate a random password and print it to stdout
		randomBytes := make([]byte, 16)
		if _, err := rand.Read(randomBytes); err != nil {
			return fmt.Errorf("failed to generate random admin password: %w", err)
		}
		adminPassword = hex.EncodeToString(randomBytes)
		log.Println("WARNING: ADMIN_PASSWORD not set. Generated random admin password:")
		fmt.Printf("\n========================================\n")
		fmt.Printf("  GENERATED ADMIN PASSWORD: %s\n", adminPassword)
		fmt.Printf("  Save this password! It will not be shown again.\n")
		fmt.Printf("========================================\n\n")
	}

	var existingUser models.User
	result := db.Where("email = ?", adminEmail).First(&existingUser)
	if result.Error == nil {
		// Admin already exists
		return nil
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	admin := models.User{
		Email:    adminEmail,
		Password: string(hashedPassword),
		Role:     "admin",
		Name:     "Admin User",
	}

	if err := db.Create(&admin).Error; err != nil {
		return err
	}

	log.Printf("Default admin created: %s", adminEmail)
	return nil
}

func CreateDefaultFranchise(db *gorm.DB) error {
	// Check if a franchise already exists
	var count int64
	db.Model(&models.Franchise{}).Count(&count)
	if count > 0 {
		return nil
	}

	// Find the admin user to assign as owner
	var admin models.User
	if err := db.Where("role = ?", "admin").First(&admin).Error; err != nil {
		log.Printf("No admin user found to assign as franchise owner, skipping default franchise creation")
		return nil
	}

	franchise := models.Franchise{
		Name:            "Grabbi Main Store",
		Slug:            "grabbi-main",
		OwnerID:         admin.ID,
		Address:         "London, UK",
		City:            "London",
		PostCode:        "EC1A 1BB",
		Latitude:        51.5074,
		Longitude:       -0.1278,
		DeliveryRadius:  5,
		DeliveryFee:     4.99,
		FreeDeliveryMin: 50,
		IsActive:        true,
	}

	if err := db.Create(&franchise).Error; err != nil {
		return fmt.Errorf("failed to create default franchise: %w", err)
	}

	// Create default store hours (Mon-Sun, 9am-9pm)
	for day := 0; day <= 6; day++ {
		hours := models.StoreHours{
			FranchiseID: franchise.ID,
			DayOfWeek:   day,
			OpenTime:    "09:00",
			CloseTime:   "21:00",
			IsClosed:    false,
		}
		db.Create(&hours)
	}

	// Backfill all existing products into FranchiseProduct for this franchise
	var products []models.Product
	db.Find(&products)
	for _, product := range products {
		fp := models.FranchiseProduct{
			FranchiseID:   franchise.ID,
			ProductID:     product.ID,
			StockQuantity: product.StockQuantity,
			ReorderLevel:  product.ReorderLevel,
			ShelfLocation: product.ShelfLocation,
			IsAvailable:   product.Status == "active",
		}
		db.Create(&fp)
	}

	// Associate all existing orders with this franchise
	db.Exec("UPDATE orders SET franchise_id = ? WHERE franchise_id IS NULL", franchise.ID)

	log.Printf("Default franchise created: %s (ID: %s)", franchise.Name, franchise.ID)
	return nil
}

func repairOrderItemsPrimaryKey(db *gorm.DB) error {
	// Ensure `order_items.id` exists and has a UUID default. This is safe to run repeatedly.
	if err := db.Exec(`
		ALTER TABLE IF EXISTS order_items
		ADD COLUMN IF NOT EXISTS id uuid;
	`).Error; err != nil {
		return fmt.Errorf("failed to ensure order_items.id column: %w", err)
	}

	if err := db.Exec(`
		ALTER TABLE IF EXISTS order_items
		ALTER COLUMN id SET DEFAULT gen_random_uuid();
	`).Error; err != nil {
		return fmt.Errorf("failed to set default for order_items.id: %w", err)
	}

	if err := db.Exec(`
		UPDATE order_items
		SET id = gen_random_uuid()
		WHERE id IS NULL;
	`).Error; err != nil {
		return fmt.Errorf("failed to backfill order_items.id: %w", err)
	}

	// Ensure the primary key is exactly (id). If an older schema set the PK to order_id
	// (or something else), multiple items per order will always fail.
	if err := db.Exec(`
DO $$
DECLARE
  pk_name text;
  pk_cols text;
BEGIN
  IF to_regclass('public.order_items') IS NULL THEN
    RETURN;
  END IF;

  SELECT c.conname,
         string_agg(a.attname, ',' ORDER BY a.attnum)
    INTO pk_name, pk_cols
    FROM pg_constraint c
    JOIN unnest(c.conkey) AS cols(attnum) ON TRUE
    JOIN pg_attribute a
      ON a.attrelid = c.conrelid AND a.attnum = cols.attnum
   WHERE c.conrelid = 'order_items'::regclass
     AND c.contype = 'p'
   GROUP BY c.conname;

  IF pk_name IS NULL THEN
    EXECUTE 'ALTER TABLE order_items ADD CONSTRAINT order_items_pkey PRIMARY KEY (id)';
  ELSIF pk_cols <> 'id' THEN
    EXECUTE format('ALTER TABLE order_items DROP CONSTRAINT %I', pk_name);
    EXECUTE 'ALTER TABLE order_items ADD CONSTRAINT order_items_pkey PRIMARY KEY (id)';
  END IF;
END $$;
	`).Error; err != nil {
		return fmt.Errorf("failed to repair order_items primary key: %w", err)
	}

	return nil
}
