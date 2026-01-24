package database

import (
	"fmt"
	"log"
	"os"

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
		&models.Product{},
		&models.ProductImage{},
		&models.CartItem{},
		&models.Order{},
		&models.OrderItem{},
		&models.Promotion{},
	); err != nil {
		return err
	}

	// AutoMigrate will NOT fix an existing incorrect primary key. If the DB was created
	// with a wrong PK on order_items (e.g. order_id), inserts will fail with:
	// "duplicate key value violates unique constraint order_items_pkey".
	if err := repairOrderItemsPrimaryKey(db); err != nil {
		return err
	}

	return nil
}

func CreateDefaultAdmin(db *gorm.DB) error {
	adminEmail := os.Getenv("ADMIN_EMAIL")
	adminPassword := os.Getenv("ADMIN_PASSWORD")

	if adminEmail == "" {
		adminEmail = "admin@grabbi.com"
	}
	if adminPassword == "" {
		adminPassword = "admin123"
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
