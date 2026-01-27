package handlers

import (
	"net/http"

	"grabbi-backend/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type OrderHandler struct {
	DB *gorm.DB
}

func (h *OrderHandler) CreateOrder(c *gin.Context) {
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

	// Get cart items with product data
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
		// Get current primary image for this product
		var primaryImage models.ProductImage
		h.DB.Where("product_id = ? AND is_primary = ?", item.ProductID, true).First(&primaryImage)

		currentPrice := item.Product.GetCurrentPrice()
		itemTotal := currentPrice * float64(item.Quantity)
		subtotal += itemTotal

		orderItems = append(orderItems, models.OrderItem{
			ID:        uuid.Nil, // Let DB default (gen_random_uuid) assign UUID
			ProductID: item.ProductID,
			ImageURL:  primaryImage.ImageURL,
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

	// Create order (without items first)
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

	// Create order items (inside the transaction).
	for i := range orderItems {
		orderItems[i].OrderID = order.ID
		orderItems[i].ID = uuid.Nil
	}

	// Omit associations to ensure we don't accidentally create Product/Order records.
	if err := tx.Omit("Product", "Order").CreateInBatches(&orderItems, 100).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create order items"})
		return
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
	h.DB.Preload("Items").Preload("Items.Product").Preload("Items.Product.Category").Preload("Items.Product.Images").Preload("User").First(&order, order.ID)

	c.JSON(http.StatusCreated, order)
}

func (h *OrderHandler) GetOrders(c *gin.Context) {
	userID, exists := c.Get("user_id")
	userRole, _ := c.Get("user_role")

	var orders []models.Order
	query := h.DB.Preload("Items").Preload("Items.Product").Preload("Items.Product.Category").Preload("Items.Product.Images").Preload("User")

	// If not admin, filter by user
	if userRole != "admin" && exists {
		query = query.Where("user_id = ?", userID)
	}

	if err := query.Order("created_at DESC").Find(&orders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch orders"})
		return
	}

	c.JSON(http.StatusOK, orders)
}

func (h *OrderHandler) GetOrder(c *gin.Context) {
	id := c.Param("id")
	userID, _ := c.Get("user_id")
	userRole, _ := c.Get("user_role")

	var order models.Order
	query := h.DB.Preload("Items").Preload("Items.Product").Preload("Items.Product.Category").Preload("Items.Product.Images").Preload("User")

	if userRole == "admin" {
		query = query.Where("id = ?", id)
	} else {
		query = query.Where("id = ? AND user_id = ?", id, userID)
	}

	if err := query.First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	c.JSON(http.StatusOK, order)
}

func (h *OrderHandler) UpdateOrderStatus(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Status models.OrderStatus `json:"status" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var order models.Order
	if err := h.DB.Where("id = ?", id).First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	order.Status = req.Status
	if err := h.DB.Save(&order).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order status"})
		return
	}

	h.DB.Preload("Items").Preload("Items.Product").Preload("Items.Product.Images").Preload("User").First(&order, order.ID)
	c.JSON(http.StatusOK, order)
}
