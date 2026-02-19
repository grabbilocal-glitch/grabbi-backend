package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Product struct {
	ID               uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	SKU              string    `gorm:"uniqueIndex;not null" json:"sku"`    // Stock Keeping Unit
	ItemName         string    `gorm:"not null;index" json:"item_name"`    // Replaces Name field
	ShortDescription string    `gorm:"type:text" json:"short_description"` // Brief product description
	LongDescription  string    `gorm:"type:text" json:"long_description"`  // Detailed product description

	// Pricing
	CostPrice      float64    `gorm:"not null" json:"cost_price"`                    // Wholesale cost price
	RetailPrice    float64    `gorm:"not null" json:"retail_price"`                  // Replaces Price field
	PromotionPrice *float64   `gorm:"default:null" json:"promotion_price,omitempty"` // Optional promotion price
	PromotionStart *time.Time `gorm:"default:null" json:"promotion_start,omitempty"` // Promotion start date
	PromotionEnd   *time.Time `gorm:"default:null" json:"promotion_end,omitempty"`   // Promotion end date

	// Margin and Tax (manually entered by admin)
	GrossMargin   float64 `gorm:"default:0" json:"gross_margin"`   // Manual gross margin percentage
	StaffDiscount float64 `gorm:"default:0" json:"staff_discount"` // Staff discount percentage
	TaxRate       float64 `gorm:"default:0" json:"tax_rate"`       // Manual tax rate percentage

	// Product Identifiers
	BatchNumber string  `gorm:"index" json:"batch_number"`                  // Batch number
	Barcode     *string `gorm:"uniqueIndex;index" json:"barcode,omitempty"` // Product barcode

	// Inventory
	StockQuantity int    `gorm:"default:0;index" json:"stock_quantity"` // Replaces Stock field
	ReorderLevel  int    `gorm:"default:0" json:"reorder_level"`        // Reorder trigger level
	ShelfLocation string `gorm:"index" json:"shelf_location"`           // Shelf location in store

	// Product Attributes
	WeightVolume  float64    `gorm:"default:0" json:"weight_volume"`                  // Weight or volume value
	UnitOfMeasure string     `json:"unit_of_measure"`                                 // Unit (kg, g, L, ml, etc.)
	ExpiryDate    *time.Time `gorm:"default:null;index" json:"expiry_date,omitempty"` // Product expiry date

	// Classification
	CategoryID      uuid.UUID    `gorm:"type:uuid;not null;index" json:"category_id"`
	Category        Category     `gorm:"foreignKey:CategoryID" json:"category,omitempty"`
	SubcategoryID   *uuid.UUID   `gorm:"type:uuid;index" json:"subcategory_id,omitempty"`       // Optional subcategory
	Subcategory     *Subcategory `gorm:"foreignKey:SubcategoryID" json:"subcategory,omitempty"` // Subcategory relationship
	Brand           string       `gorm:"index" json:"brand"`                                    // Brand name
	Supplier        string       `gorm:"index" json:"supplier"`                                 // Supplier name
	CountryOfOrigin string       `json:"country_of_origin"`                                     // Origin country

	// Dietary and Restrictions
	IsGlutenFree    bool `gorm:"default:false;index" json:"is_gluten_free"`    // Gluten free status
	IsVegetarian    bool `gorm:"default:false;index" json:"is_vegetarian"`     // Vegetarian status
	IsVegan         bool `gorm:"default:false;index" json:"is_vegan"`          // Vegan status
	IsAgeRestricted bool `gorm:"default:false;index" json:"is_age_restricted"` // Age restricted flag
	MinimumAge      *int `gorm:"default:null" json:"minimum_age,omitempty"`    // Minimum age for age-restricted items

	// Additional Info
	AllergenInfo  string `gorm:"type:text" json:"allergen_info"`           // Allergen information
	StorageType   string `gorm:"index" json:"storage_type"`                // Storage requirement (refrigerated, frozen, etc.)
	IsOwnBrand    bool   `gorm:"default:false;index" json:"is_own_brand"`  // Own brand flag
	OnlineVisible bool   `gorm:"default:true;index" json:"online_visible"` // Visibility on online platform
	Status        string `gorm:"default:active;index" json:"status"`       // Product status (active, inactive)
	Notes         string `gorm:"type:text" json:"notes"`                   // Additional notes
	PackSize      string `json:"pack_size"`

	// Timestamps
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	// Deletion tracking
	DeletedBy string `gorm:"default:null" json:"deleted_by,omitempty"`

	// Relations
	Images []ProductImage `gorm:"foreignKey:ProductID" json:"images,omitempty"`
}

func (p *Product) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// IsPromotionActive returns true if a promotion is currently active
func (p *Product) IsPromotionActive() bool {
	if p.PromotionPrice == nil {
		return false
	}

	now := time.Now()

	// If no dates set, promotion is inactive
	if p.PromotionStart == nil && p.PromotionEnd == nil {
		return false
	}

	// Check start date
	if p.PromotionStart != nil && now.Before(*p.PromotionStart) {
		return false
	}

	// Check end date
	if p.PromotionEnd != nil && now.After(*p.PromotionEnd) {
		return false
	}

	return true
}

// GetCurrentPrice returns the current price (promotion price if active, otherwise retail price)
func (p *Product) GetCurrentPrice() float64 {
	if p.IsPromotionActive() && p.PromotionPrice != nil {
		return *p.PromotionPrice
	}
	return p.RetailPrice
}
