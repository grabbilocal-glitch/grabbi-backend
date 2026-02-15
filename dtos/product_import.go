package dtos

import "github.com/google/uuid"

// ProductImportRequest represents the request structure for batch product import
type ProductImportRequest struct {
	Products      []ProductImportItem `json:"products" binding:"required,min=1,max=5000"`
	DeleteMissing bool                `json:"delete_missing"` // If true, delete products not in import (default false)
}

// ProductImportItem represents a single product item in the import
type ProductImportItem struct {
	ID                *string  `json:"id"`
	SKU               string   `json:"sku"`
	ItemName          string   `json:"item_name" binding:"required"`
	ShortDescription  string   `json:"short_description"`
	LongDescription   string   `json:"long_description"`
	CostPrice         float64  `json:"cost_price" binding:"required,min=0.01"`
	RetailPrice       float64  `json:"retail_price" binding:"required,min=0.01"`
	PromotionPrice    *float64 `json:"promotion_price"`
	PromotionStart    string   `json:"promotion_start"`
	PromotionEnd      string   `json:"promotion_end"`
	GrossMargin       float64  `json:"gross_margin"`
	StaffDiscount     float64  `json:"staff_discount"`
	TaxRate           float64  `json:"tax_rate"`
	StockQuantity     int      `json:"stock_quantity" binding:"required,min=0"`
	ReorderLevel      int      `json:"reorder_level" binding:"required,min=0"`
	ShelfLocation     string   `json:"shelf_location"`
	WeightVolume      float64  `json:"weight_volume"`
	UnitOfMeasure     string   `json:"unit_of_measure"`
	ExpiryDate        string   `json:"expiry_date"`
	CategoryID        string   `json:"category_id" binding:"required"`
	SubcategoryID     *string  `json:"subcategory_id"`
	Brand             string   `json:"brand"`
	Supplier          string   `json:"supplier"`
	CountryOfOrigin   string   `json:"country_of_origin"`
	IsGlutenFree      bool     `json:"is_gluten_free"`
	IsVegetarian      bool     `json:"is_vegetarian"`
	IsVegan           bool     `json:"is_vegan"`
	IsAgeRestricted   bool     `json:"is_age_restricted"`
	MinimumAge        *int     `json:"minimum_age"`
	AllergenInfo      string   `json:"allergen_info"`
	StorageType       string   `json:"storage_type"`
	IsOwnBrand        bool     `json:"is_own_brand"`
	OnlineVisible     bool     `json:"online_visible"`
	Status            string   `json:"status"`
	Barcode           string   `json:"barcode"`
	BatchNumber       string   `json:"batch_number"`
	PackSize          string   `json:"pack_size"`
	Notes             string   `json:"notes"`
	ImageURLs         []string `json:"image_urls"`
	ImagesProvided    bool     `json:"images_provided"`
	FranchiseIDs      []string `json:"franchise_ids"`
	Delete            bool     `json:"delete"`
}

// ProductOrderCount represents the count of orders per product (for deletion safety)
type ProductOrderCount struct {
	ProductID uuid.UUID `json:"product_id"`
	Count     int64     `json:"count"`
}

// ImageOrderCount represents the count of orders per image URL (for deletion safety)
type ImageOrderCount struct {
	ImageURL string `json:"image_url"`
	Count    int64  `json:"count"`
}
