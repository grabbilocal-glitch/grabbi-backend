package handlers

import (
	"fmt"
	"net/http"
	"time"

	"grabbi-backend/models"
	"grabbi-backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
		DeliveryAddress string   `json:"delivery_address" binding:"required"`
		PaymentMethod   string   `json:"payment_method"`
		FranchiseID     string   `json:"franchise_id"`
		CustomerLat     *float64 `json:"customer_lat"`
		CustomerLng     *float64 `json:"customer_lng"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": utils.SanitizeValidationError(err)})
		return
	}

	// Determine franchise
	var franchiseID *uuid.UUID
	var franchise *models.Franchise

	if req.FranchiseID != "" {
		fID, err := uuid.Parse(req.FranchiseID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid franchise_id"})
			return
		}
		var f models.Franchise
		if err := h.DB.Where("id = ? AND is_active = ?", fID, true).First(&f).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Franchise not found"})
			return
		}
		franchiseID = &fID
		franchise = &f
	} else if req.CustomerLat != nil && req.CustomerLng != nil {
		// Find nearest franchise
		var franchises []models.Franchise
		h.DB.Where("is_active = ?", true).Find(&franchises)

		var nearest *models.Franchise
		var nearestDist float64 = -1
		for i := range franchises {
			dist := utils.Haversine(*req.CustomerLat, *req.CustomerLng, franchises[i].Latitude, franchises[i].Longitude)
			if dist <= franchises[i].DeliveryRadius && (nearestDist < 0 || dist < nearestDist) {
				nearest = &franchises[i]
				nearestDist = dist
			}
		}

		if nearest == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No franchise delivers to your location"})
			return
		}
		franchiseID = &nearest.ID
		franchise = nearest
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

	// Batch query all primary images for products in the cart
	productIDs := make([]uuid.UUID, len(cartItems))
	for i, item := range cartItems {
		productIDs[i] = item.ProductID
	}
	var primaryImages []models.ProductImage
	h.DB.Where("product_id IN ? AND is_primary = ?", productIDs, true).Find(&primaryImages)

	// Build a map for quick lookup
	primaryImageMap := make(map[uuid.UUID]string)
	for _, img := range primaryImages {
		primaryImageMap[img.ProductID] = img.ImageURL
	}

	// Calculate totals
	var subtotal float64
	var orderItems []models.OrderItem

	for _, item := range cartItems {
		imageURL := primaryImageMap[item.ProductID]

		currentPrice := item.Product.GetCurrentPrice()

		// If franchise specified, check for price overrides
		if franchiseID != nil {
			var fp models.FranchiseProduct
			if err := h.DB.Where("franchise_id = ? AND product_id = ?", franchiseID, item.ProductID).First(&fp).Error; err == nil {
				if fp.RetailPriceOverride != nil {
					currentPrice = *fp.RetailPriceOverride
				}
				// Check franchise promo override
				if fp.PromotionPriceOverride != nil {
					currentPrice = *fp.PromotionPriceOverride
				}
			}
		}

		itemTotal := currentPrice * float64(item.Quantity)
		subtotal += itemTotal

		orderItems = append(orderItems, models.OrderItem{
			ID:        uuid.Nil,
			ProductID: item.ProductID,
			ImageURL:  imageURL,
			Quantity:  item.Quantity,
			Price:     currentPrice,
		})
	}

	// Calculate delivery fee from franchise settings or defaults
	deliveryFee := 0.0
	freeThreshold := 20.0
	if franchise != nil {
		if subtotal < franchise.FreeDeliveryMin {
			deliveryFee = franchise.DeliveryFee
		}
		freeThreshold = franchise.FreeDeliveryMin
	} else {
		if subtotal < freeThreshold {
			deliveryFee = 3.75
		}
	}
	_ = freeThreshold

	total := subtotal + deliveryFee
	pointsEarned := int(subtotal)

	// Create order
	order := models.Order{
		ID:              uuid.New(),
		UserID:          userID.(uuid.UUID),
		FranchiseID:     franchiseID,
		Status:          models.OrderStatusPending,
		Subtotal:        subtotal,
		DeliveryFee:     deliveryFee,
		Total:           total,
		DeliveryAddress: req.DeliveryAddress,
		PaymentMethod:   req.PaymentMethod,
		PointsEarned:    pointsEarned,
		CustomerLat:     req.CustomerLat,
		CustomerLng:     req.CustomerLng,
	}

	// Start transaction
	tx := h.DB.Begin()

	// Update stock with row-level locking to prevent race conditions
	for _, item := range cartItems {
		if franchiseID != nil {
			var fp models.FranchiseProduct
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("franchise_id = ? AND product_id = ?", franchiseID, item.ProductID).
				First(&fp).Error; err == nil {
				if fp.StockQuantity < item.Quantity {
					tx.Rollback()
					c.JSON(http.StatusBadRequest, gin.H{"error": "Insufficient stock for " + item.Product.ItemName})
					return
				}
				fp.StockQuantity -= item.Quantity
				tx.Save(&fp)
				continue
			}
		}

		// Fallback to master product stock with row-level locking
		var product models.Product
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", item.ProductID).
			First(&product).Error; err != nil {
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

	// Create order items
	for i := range orderItems {
		orderItems[i].OrderID = order.ID
		orderItems[i].ID = uuid.Nil
	}

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

	// Send order confirmation email (non-blocking)
	utils.SendOrderConfirmation(user.Email, user.Name, order.OrderNumber, order.Total)

	c.JSON(http.StatusCreated, order)
}

func (h *OrderHandler) GetOrders(c *gin.Context) {
	userID, exists := c.Get("user_id")
	userRole, _ := c.Get("user_role")

	var orders []models.Order
	query := h.DB.Preload("Items").Preload("Items.Product").Preload("Items.Product.Category").Preload("Items.Product.Images").Preload("User")

	roleStr, _ := userRole.(string)

	switch roleStr {
	case "admin":
		// Admin sees all orders, optionally filtered by franchise
		if fID := c.Query("franchise_id"); fID != "" {
			query = query.Where("franchise_id = ?", fID)
		}
	case "franchise_owner", "franchise_staff":
		// Franchise roles see only their franchise's orders
		if fID, ok := c.Get("franchise_id"); ok {
			query = query.Where("franchise_id = ?", fID)
		}
	default:
		// Regular customer sees their own orders
		if exists {
			query = query.Where("user_id = ?", userID)
		}
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

	roleStr, _ := userRole.(string)

	switch roleStr {
	case "admin":
		query = query.Where("id = ?", id)
	case "franchise_owner", "franchise_staff":
		if fID, ok := c.Get("franchise_id"); ok {
			query = query.Where("id = ? AND franchise_id = ?", id, fID)
		}
	default:
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
	userRole, _ := c.Get("user_role")

	var req struct {
		Status models.OrderStatus `json:"status" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": utils.SanitizeValidationError(err)})
		return
	}

	var order models.Order
	query := h.DB.Where("id = ?", id)

	// Franchise roles can only update their franchise's orders
	roleStr, _ := userRole.(string)
	if roleStr == "franchise_owner" || roleStr == "franchise_staff" {
		if fID, ok := c.Get("franchise_id"); ok {
			query = query.Where("franchise_id = ?", fID)
		}
	}

	if err := query.First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	// Validate state transition
	if !models.IsValidTransition(order.Status, req.Status) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Invalid status transition from '%s' to '%s'", order.Status, req.Status),
		})
		return
	}

	order.Status = req.Status
	if err := h.DB.Save(&order).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order status"})
		return
	}

	// Restore stock on cancellation
	if req.Status == models.OrderStatusCancelled {
		var items []models.OrderItem
		h.DB.Where("order_id = ?", order.ID).Find(&items)
		for _, item := range items {
			if order.FranchiseID != nil {
				var fp models.FranchiseProduct
				if err := h.DB.Where("franchise_id = ? AND product_id = ?", order.FranchiseID, item.ProductID).First(&fp).Error; err == nil {
					fp.StockQuantity += item.Quantity
					h.DB.Save(&fp)
					continue
				}
			}
			// Fallback to master product stock
			h.DB.Model(&models.Product{}).Where("id = ?", item.ProductID).
				Update("stock_quantity", gorm.Expr("stock_quantity + ?", item.Quantity))
		}
	}

	h.DB.Preload("Items").Preload("Items.Product").Preload("Items.Product.Images").Preload("User").First(&order, order.ID)

	// Send status update email (non-blocking)
	if order.User.Email != "" {
		utils.SendOrderStatusUpdate(order.User.Email, order.User.Name, order.OrderNumber, string(req.Status))
	}

	c.JSON(http.StatusOK, order)
}

func (h *OrderHandler) GetOrderTransitions(c *gin.Context) {
	c.JSON(http.StatusOK, models.AllowedTransitions)
}

// GetAdminDashboard returns pre-computed dashboard stats for admin with optional franchise filter
func (h *OrderHandler) GetAdminDashboard(c *gin.Context) {
	franchiseID := c.Query("franchise_id")

	// Product count
	var productCount int64
	if franchiseID != "" {
		h.DB.Model(&models.FranchiseProduct{}).Where("franchise_id = ?", franchiseID).Count(&productCount)
	} else {
		h.DB.Model(&models.Product{}).Count(&productCount)
	}

	// Order stats
	orderQuery := h.DB.Model(&models.Order{})
	if franchiseID != "" {
		orderQuery = orderQuery.Where("franchise_id = ?", franchiseID)
	}

	var totalOrders int64
	orderQuery.Count(&totalOrders)

	var totalRevenue float64
	revenueQuery := h.DB.Model(&models.Order{})
	if franchiseID != "" {
		revenueQuery = revenueQuery.Where("franchise_id = ?", franchiseID)
	}
	revenueQuery.Select("COALESCE(SUM(total), 0)").Scan(&totalRevenue)

	// Recent revenue (last 7 days)
	sevenDaysAgo := time.Now().AddDate(0, 0, -7)
	var recentRevenue float64
	recentQuery := h.DB.Model(&models.Order{}).Where("created_at >= ?", sevenDaysAgo)
	if franchiseID != "" {
		recentQuery = recentQuery.Where("franchise_id = ?", franchiseID)
	}
	recentQuery.Select("COALESCE(SUM(total), 0)").Scan(&recentRevenue)

	// Pending orders
	var pendingOrders int64
	pendingQuery := h.DB.Model(&models.Order{}).Where("status = ?", "pending")
	if franchiseID != "" {
		pendingQuery = pendingQuery.Where("franchise_id = ?", franchiseID)
	}
	pendingQuery.Count(&pendingOrders)

	// Category count
	var categoryCount int64
	h.DB.Model(&models.Category{}).Count(&categoryCount)

	// Franchise count
	var franchiseCount int64
	h.DB.Model(&models.Franchise{}).Count(&franchiseCount)

	// Recent orders
	var recentOrders []models.Order
	recentOrdersQuery := h.DB.Preload("Items").Preload("User").Order("created_at DESC").Limit(10)
	if franchiseID != "" {
		recentOrdersQuery = recentOrdersQuery.Where("franchise_id = ?", franchiseID)
	}
	recentOrdersQuery.Find(&recentOrders)

	c.JSON(http.StatusOK, gin.H{
		"total_products":  productCount,
		"total_orders":    totalOrders,
		"total_revenue":   totalRevenue,
		"recent_revenue":  recentRevenue,
		"pending_orders":  pendingOrders,
		"total_categories": categoryCount,
		"total_franchises": franchiseCount,
		"recent_orders":   recentOrders,
	})
}
