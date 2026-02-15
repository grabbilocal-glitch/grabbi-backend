package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type FranchiseProduct struct {
	ID                     uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	FranchiseID            uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex:idx_franchise_product" json:"franchise_id"`
	Franchise              Franchise  `gorm:"foreignKey:FranchiseID" json:"-"`
	ProductID              uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex:idx_franchise_product" json:"product_id"`
	Product                Product    `gorm:"foreignKey:ProductID" json:"product,omitempty"`
	RetailPriceOverride    *float64   `json:"retail_price_override"`
	PromotionPriceOverride *float64   `json:"promotion_price_override"`
	PromotionStartOverride *time.Time `json:"promotion_start_override"`
	PromotionEndOverride   *time.Time `json:"promotion_end_override"`
	StockQuantity          int        `gorm:"default:0" json:"stock_quantity"`
	ReorderLevel           int        `gorm:"default:5" json:"reorder_level"`
	ShelfLocation          string     `json:"shelf_location"`
	IsAvailable            bool       `gorm:"default:true" json:"is_available"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

func (fp *FranchiseProduct) BeforeCreate(tx *gorm.DB) error {
	if fp.ID == uuid.Nil {
		fp.ID = uuid.New()
	}
	return nil
}
