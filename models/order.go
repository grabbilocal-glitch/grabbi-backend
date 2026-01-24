package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusConfirmed OrderStatus = "confirmed"
	OrderStatusPreparing OrderStatus = "preparing"
	OrderStatusReady     OrderStatus = "ready"
	OrderStatusOutForDelivery OrderStatus = "out_for_delivery"
	OrderStatusDelivered OrderStatus = "delivered"
	OrderStatusCancelled OrderStatus = "cancelled"
)

type Order struct {
	ID              uuid.UUID   `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID          uuid.UUID   `gorm:"type:uuid;not null" json:"user_id"`
	User            User        `gorm:"foreignKey:UserID" json:"user,omitempty"`
	OrderNumber     string      `gorm:"uniqueIndex;not null" json:"order_number"`
	Status          OrderStatus `gorm:"default:pending" json:"status"`
	Subtotal        float64     `gorm:"not null" json:"subtotal"`
	DeliveryFee     float64     `gorm:"default:0" json:"delivery_fee"`
	Total           float64     `gorm:"not null" json:"total"`
	DeliveryAddress string      `json:"delivery_address"`
	PaymentMethod   string      `json:"payment_method"`
	PointsEarned    int         `gorm:"default:0" json:"points_earned"`
	Items           []OrderItem `gorm:"foreignKey:OrderID" json:"items"`
	CreatedAt       time.Time   `json:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}

type OrderItem struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	OrderID   uuid.UUID `gorm:"type:uuid;not null" json:"order_id"`
	Order     Order     `gorm:"foreignKey:OrderID" json:"-"`
	ProductID uuid.UUID `gorm:"type:uuid;not null;index" json:"product_id"`
	Product   Product   `gorm:"foreignKey:ProductID" json:"product"`
	ImageURL  string    `json:"image_url"`
	Quantity  int       `gorm:"not null" json:"quantity"`
	Price     float64   `gorm:"not null" json:"price"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (o *Order) BeforeCreate(tx *gorm.DB) error {
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}
	if o.OrderNumber == "" {
		o.OrderNumber = "ORD" + time.Now().Format("20060102150405") + o.ID.String()[:8]
	}
	return nil
}

// BeforeCreate hook removed - using database-generated UUIDs via raw SQL
