package handlers

import (
	"net/http"

	"grabbi-backend/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Fixed CreateOrder function - replace the one in order.go
func (h *OrderHandler) CreateOrderFixed(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req struct {
		DeliveryAddress string `json:"delivery_address" binding:"required"`
		PaymentMethod   string `json:"payment_method"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get cart items
	var cartItems []models.CartItem
	if err := h.DB.Preload("Product").Where("user_id = ?", userID).Find(&cartItems).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch cart"})
		return
	}

	if len(cartItems) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cart is empty"})
		return
	}

	// Calculate totals
	var subtotal float64
	var orderItems []models.OrderItem

	for _, item := range cartItems {
		currentPrice := item.Product.GetCurrentPrice()
		itemTotal := currentPrice * float64(item.Quantity)
		subtotal += itemTotal

		orderItems = append(orderItems, models.OrderItem{
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
			Price:     currentPrice,
		})
	}

	// Calculate delivery fee (free if subtotal >= 20)
	deliveryFee := 0.0
	if subtotal < 20 {
		deliveryFee = 3.75
	}

	total := subtotal + deliveryFee
	pointsEarned := int(subtotal)

	// Create order
	order := models.Order{
		ID:              uuid.New(),
		UserID:          userID.(uuid.UUID),
		Status:          models.OrderStatusPending,
		Subtotal:        subtotal,
		DeliveryFee:     deliveryFee,
		Total:           total,
		DeliveryAddress: req.DeliveryAddress,
		PaymentMethod:   req.PaymentMethod,
		PointsEarned:    pointsEarned,
	}

	// Start transaction
	tx := h.DB.Begin()

	// Update product stock
	for _, item := range cartItems {
		var product models.Product
		if err := tx.Where("id = ?", item.ProductID).First(&product).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Product not found"})
			return
		}

		if product.StockQuantity < item.Quantity {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{"error": "Insufficient stock for " + product.ItemName})
			return
		}

		product.StockQuantity -= item.Quantity
		tx.Save(&product)
	}

	// Create order
	if err := tx.Create(&order).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create order"})
		return
	}

	// Create order items using direct SQL connection to bypass GORM hooks
	sqlDB, err := tx.DB()
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get database connection"})
		return
	}

	// Insert order items using raw SQL with database-generated UUIDs
	for _, item := range orderItems {
		_, err := sqlDB.Exec(`
			INSERT INTO order_items (id, order_id, product_id, quantity, price, created_at, updated_at)
			VALUES (gen_random_uuid(), $1, $2, $3, $4, NOW(), NOW())
		`, order.ID, item.ProductID, item.Quantity, item.Price)

		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create order items: " + err.Error()})
			return
		}
	}

	// Update user loyalty points
	var user models.User
	tx.Where("id = ?", userID).First(&user)
	user.LoyaltyPoints += pointsEarned
	tx.Save(&user)

	// Clear cart
	tx.Where("user_id = ?", userID).Delete(&models.CartItem{})

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to complete order"})
		return
	}

	// Load order with relations
	var loadedOrder models.Order
	if err := h.DB.Preload("Items").Preload("Items.Product").Preload("Items.Product.Category").Preload("User").First(&loadedOrder, order.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Order created but failed to load: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, loadedOrder)
}
