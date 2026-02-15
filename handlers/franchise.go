package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"grabbi-backend/models"
	"grabbi-backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type FranchiseHandler struct {
	DB *gorm.DB
}

// ========== Public Endpoints ==========

func (h *FranchiseHandler) GetNearestFranchise(c *gin.Context) {
	latStr := c.Query("lat")
	lngStr := c.Query("lng")

	if latStr == "" || lngStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "lat and lng query parameters are required"})
		return
	}

	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid latitude"})
		return
	}

	lng, err := strconv.ParseFloat(lngStr, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid longitude"})
		return
	}

	var franchises []models.Franchise
	if err := h.DB.Preload("StoreHours").Where("is_active = ?", true).Find(&franchises).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch franchises"})
		return
	}

	var nearest *models.Franchise
	var nearestDistance float64 = -1

	for i := range franchises {
		dist := utils.Haversine(lat, lng, franchises[i].Latitude, franchises[i].Longitude)
		if dist <= franchises[i].DeliveryRadius && (nearestDistance < 0 || dist < nearestDistance) {
			nearest = &franchises[i]
			nearestDistance = dist
		}
	}

	if nearest == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No franchise delivers to your location"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"franchise": nearest,
		"distance":  nearestDistance,
	})
}

func (h *FranchiseHandler) GetFranchise(c *gin.Context) {
	id := c.Param("id")

	var franchise models.Franchise
	if err := h.DB.Preload("StoreHours").Where("id = ? AND is_active = ?", id, true).First(&franchise).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Franchise not found"})
		return
	}

	c.JSON(http.StatusOK, franchise)
}

func (h *FranchiseHandler) GetFranchiseProducts(c *gin.Context) {
	franchiseID := c.Param("id")

	var franchise models.Franchise
	if err := h.DB.Where("id = ?", franchiseID).First(&franchise).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Franchise not found"})
		return
	}

	// Fetch franchise products with master product data
	var fps []models.FranchiseProduct
	query := h.DB.Preload("Product").Preload("Product.Category").Preload("Product.Images").
		Where("franchise_id = ? AND is_available = ?", franchiseID, true)

	if search := c.Query("search"); search != "" {
		query = query.Joins("JOIN products ON products.id = franchise_products.product_id").
			Where("LOWER(products.item_name) LIKE LOWER(?)", "%"+search+"%")
	}

	if categoryID := c.Query("category_id"); categoryID != "" {
		query = query.Joins("JOIN products p2 ON p2.id = franchise_products.product_id").
			Where("p2.category_id = ?", categoryID)
	}

	if err := query.Find(&fps).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch products"})
		return
	}

	// Merge overrides onto master product data
	type MergedProduct struct {
		models.Product
		FranchiseStock    int      `json:"franchise_stock"`
		FranchisePrice    float64  `json:"franchise_price"`
		PromoPrice        *float64 `json:"franchise_promo_price,omitempty"`
		ShelfLocation     string   `json:"franchise_shelf_location"`
		FranchiseAvailable bool    `json:"franchise_available"`
	}

	var result []MergedProduct
	for _, fp := range fps {
		mp := MergedProduct{
			Product:            fp.Product,
			FranchiseStock:     fp.StockQuantity,
			ShelfLocation:      fp.ShelfLocation,
			FranchiseAvailable: fp.IsAvailable,
		}

		// Apply price overrides
		if fp.RetailPriceOverride != nil {
			mp.FranchisePrice = *fp.RetailPriceOverride
		} else {
			mp.FranchisePrice = fp.Product.RetailPrice
		}

		if fp.PromotionPriceOverride != nil {
			mp.PromoPrice = fp.PromotionPriceOverride
		} else if fp.Product.PromotionPrice != nil {
			mp.PromoPrice = fp.Product.PromotionPrice
		}

		result = append(result, mp)
	}

	c.JSON(http.StatusOK, result)
}

func (h *FranchiseHandler) GetFranchisePromotions(c *gin.Context) {
	franchiseID := c.Param("id")

	var promotions []models.FranchisePromotion
	now := time.Now()
	if err := h.DB.Where(
		"franchise_id = ? AND is_active = ? AND (start_date IS NULL OR start_date <= ?) AND (end_date IS NULL OR end_date >= ?)",
		franchiseID, true, now, now,
	).Find(&promotions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch promotions"})
		return
	}

	c.JSON(http.StatusOK, promotions)
}

// ========== Admin (Super Admin) Endpoints ==========

func (h *FranchiseHandler) ListFranchises(c *gin.Context) {
	var franchises []models.Franchise
	if err := h.DB.Preload("Owner").Find(&franchises).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch franchises"})
		return
	}

	// Batch query: get order counts for all franchises in a single GROUP BY query
	type orderCountResult struct {
		FranchiseID uuid.UUID `gorm:"column:franchise_id"`
		OrderCount  int64     `gorm:"column:order_count"`
	}
	var counts []orderCountResult
	h.DB.Model(&models.Order{}).
		Select("franchise_id, count(*) as order_count").
		Where("franchise_id IS NOT NULL").
		Group("franchise_id").
		Find(&counts)

	countMap := make(map[uuid.UUID]int64)
	for _, c := range counts {
		countMap[c.FranchiseID] = c.OrderCount
	}

	type FranchiseWithStats struct {
		models.Franchise
		OrderCount int64 `json:"order_count"`
	}

	var result []FranchiseWithStats
	for _, f := range franchises {
		result = append(result, FranchiseWithStats{
			Franchise:  f,
			OrderCount: countMap[f.ID],
		})
	}

	c.JSON(http.StatusOK, result)
}

func (h *FranchiseHandler) CreateFranchise(c *gin.Context) {
	var req struct {
		Name            string  `json:"name" binding:"required"`
		Slug            string  `json:"slug" binding:"required"`
		OwnerEmail      string  `json:"owner_email" binding:"required,email"`
		OwnerName       string  `json:"owner_name"`
		OwnerPassword   string  `json:"owner_password" binding:"required,min=8"`
		Address         string  `json:"address"`
		City            string  `json:"city"`
		PostCode        string  `json:"post_code"`
		Latitude        float64 `json:"latitude" binding:"required"`
		Longitude       float64 `json:"longitude" binding:"required"`
		DeliveryRadius  float64 `json:"delivery_radius"`
		DeliveryFee     float64 `json:"delivery_fee"`
		FreeDeliveryMin float64 `json:"free_delivery_min"`
		Phone           string  `json:"phone"`
		Email           string  `json:"email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": utils.SanitizeValidationError(err)})
		return
	}

	tx := h.DB.Begin()

	// Create or find the owner user (including soft-deleted users to avoid unique constraint violation)
	var owner models.User
	if err := tx.Unscoped().Where("email = ?", req.OwnerEmail).First(&owner).Error; err != nil {
		// No user found at all — create new user as franchise owner
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.OwnerPassword), bcrypt.DefaultCost)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
			return
		}

		owner = models.User{
			ID:       uuid.New(),
			Email:    req.OwnerEmail,
			Password: string(hashedPassword),
			Name:     req.OwnerName,
			Role:     "franchise_owner",
		}

		if err := tx.Create(&owner).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create owner user"})
			return
		}
	} else if owner.DeletedAt.Valid {
		// Restore soft-deleted user and update their details
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.OwnerPassword), bcrypt.DefaultCost)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
			return
		}
		if err := tx.Unscoped().Model(&owner).Updates(map[string]interface{}{
			"deleted_at": nil,
			"role":       "franchise_owner",
			"name":       req.OwnerName,
			"password":   string(hashedPassword),
		}).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to restore owner user"})
			return
		}
	}
	// else: existing active user found, reuse as-is

	franchise := models.Franchise{
		Name:            req.Name,
		Slug:            req.Slug,
		OwnerID:         owner.ID,
		Address:         req.Address,
		City:            req.City,
		PostCode:        req.PostCode,
		Latitude:        req.Latitude,
		Longitude:       req.Longitude,
		DeliveryRadius:  req.DeliveryRadius,
		DeliveryFee:     req.DeliveryFee,
		FreeDeliveryMin: req.FreeDeliveryMin,
		Phone:           req.Phone,
		Email:           req.Email,
		IsActive:        true,
	}

	if franchise.DeliveryRadius == 0 {
		franchise.DeliveryRadius = 5
	}
	if franchise.DeliveryFee == 0 {
		franchise.DeliveryFee = 4.99
	}
	if franchise.FreeDeliveryMin == 0 {
		franchise.FreeDeliveryMin = 50
	}

	if err := tx.Create(&franchise).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create franchise: " + err.Error()})
		return
	}

	// Update owner with franchise ID
	tx.Model(&owner).Update("franchise_id", franchise.ID)

	// Create default store hours
	for day := 0; day <= 6; day++ {
		hours := models.StoreHours{
			FranchiseID: franchise.ID,
			DayOfWeek:   day,
			OpenTime:    "09:00",
			CloseTime:   "21:00",
		}
		tx.Create(&hours)
	}

	// Backfill all active products as franchise products
	var products []models.Product
	tx.Where("status = ?", "active").Find(&products)
	for _, product := range products {
		fp := models.FranchiseProduct{
			FranchiseID:   franchise.ID,
			ProductID:     product.ID,
			StockQuantity: product.StockQuantity,
			ReorderLevel:  product.ReorderLevel,
			ShelfLocation: product.ShelfLocation,
			IsAvailable:   true,
		}
		tx.Create(&fp)
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to finalize franchise creation"})
		return
	}

	h.DB.Preload("Owner").Preload("StoreHours").First(&franchise, franchise.ID)
	c.JSON(http.StatusCreated, franchise)
}

func (h *FranchiseHandler) UpdateFranchise(c *gin.Context) {
	id := c.Param("id")

	var franchise models.Franchise
	if err := h.DB.Where("id = ?", id).First(&franchise).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Franchise not found"})
		return
	}

	var req struct {
		Name            *string  `json:"name"`
		Address         *string  `json:"address"`
		City            *string  `json:"city"`
		PostCode        *string  `json:"post_code"`
		Latitude        *float64 `json:"latitude"`
		Longitude       *float64 `json:"longitude"`
		DeliveryRadius  *float64 `json:"delivery_radius"`
		DeliveryFee     *float64 `json:"delivery_fee"`
		FreeDeliveryMin *float64 `json:"free_delivery_min"`
		Phone           *string  `json:"phone"`
		Email           *string  `json:"email"`
		IsActive        *bool    `json:"is_active"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": utils.SanitizeValidationError(err)})
		return
	}

	if req.Name != nil {
		franchise.Name = *req.Name
	}
	if req.Address != nil {
		franchise.Address = *req.Address
	}
	if req.City != nil {
		franchise.City = *req.City
	}
	if req.PostCode != nil {
		franchise.PostCode = *req.PostCode
	}
	if req.Latitude != nil {
		franchise.Latitude = *req.Latitude
	}
	if req.Longitude != nil {
		franchise.Longitude = *req.Longitude
	}
	if req.DeliveryRadius != nil {
		franchise.DeliveryRadius = *req.DeliveryRadius
	}
	if req.DeliveryFee != nil {
		franchise.DeliveryFee = *req.DeliveryFee
	}
	if req.FreeDeliveryMin != nil {
		franchise.FreeDeliveryMin = *req.FreeDeliveryMin
	}
	if req.Phone != nil {
		franchise.Phone = *req.Phone
	}
	if req.Email != nil {
		franchise.Email = *req.Email
	}
	if req.IsActive != nil {
		franchise.IsActive = *req.IsActive
	}

	if err := h.DB.Save(&franchise).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update franchise"})
		return
	}

	h.DB.Preload("Owner").Preload("StoreHours").First(&franchise, franchise.ID)
	c.JSON(http.StatusOK, franchise)
}

func (h *FranchiseHandler) DeleteFranchise(c *gin.Context) {
	id := c.Param("id")

	var franchise models.Franchise
	if err := h.DB.Where("id = ?", id).First(&franchise).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Franchise not found"})
		return
	}

	// Check for dependencies before deleting
	var orderCount int64
	h.DB.Model(&models.Order{}).Where("franchise_id = ?", id).Count(&orderCount)

	var staffCount int64
	h.DB.Model(&models.FranchiseStaff{}).Where("franchise_id = ?", id).Count(&staffCount)

	var productCount int64
	h.DB.Model(&models.FranchiseProduct{}).Where("franchise_id = ?", id).Count(&productCount)

	if orderCount > 0 || staffCount > 0 || productCount > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"error":         "Cannot delete franchise with existing dependencies. Consider deactivating instead.",
			"order_count":   orderCount,
			"staff_count":   staffCount,
			"product_count": productCount,
		})
		return
	}

	if err := h.DB.Delete(&franchise).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete franchise"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Franchise deleted successfully"})
}

func (h *FranchiseHandler) GetFranchiseOrders(c *gin.Context) {
	franchiseID := c.Param("id")

	var orders []models.Order
	if err := h.DB.Preload("Items").Preload("Items.Product").Preload("User").
		Where("franchise_id = ?", franchiseID).
		Order("created_at DESC").
		Find(&orders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch orders"})
		return
	}

	c.JSON(http.StatusOK, orders)
}

// ========== Franchise Portal Endpoints ==========

func (h *FranchiseHandler) GetMyFranchise(c *gin.Context) {
	franchiseID, _ := c.Get("franchise_id")

	var franchise models.Franchise
	if err := h.DB.Preload("StoreHours").Preload("Staff").Preload("Staff.User").
		Where("id = ?", franchiseID).First(&franchise).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Franchise not found"})
		return
	}

	c.JSON(http.StatusOK, franchise)
}

func (h *FranchiseHandler) UpdateMyFranchise(c *gin.Context) {
	franchiseID, _ := c.Get("franchise_id")

	var franchise models.Franchise
	if err := h.DB.Where("id = ?", franchiseID).First(&franchise).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Franchise not found"})
		return
	}

	var req struct {
		Address         *string  `json:"address"`
		City            *string  `json:"city"`
		PostCode        *string  `json:"post_code"`
		Phone           *string  `json:"phone"`
		Email           *string  `json:"email"`
		DeliveryRadius  *float64 `json:"delivery_radius"`
		DeliveryFee     *float64 `json:"delivery_fee"`
		FreeDeliveryMin *float64 `json:"free_delivery_min"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": utils.SanitizeValidationError(err)})
		return
	}

	if req.Address != nil {
		franchise.Address = *req.Address
	}
	if req.City != nil {
		franchise.City = *req.City
	}
	if req.PostCode != nil {
		franchise.PostCode = *req.PostCode
	}
	if req.Phone != nil {
		franchise.Phone = *req.Phone
	}
	if req.Email != nil {
		franchise.Email = *req.Email
	}
	if req.DeliveryRadius != nil {
		franchise.DeliveryRadius = *req.DeliveryRadius
	}
	if req.DeliveryFee != nil {
		franchise.DeliveryFee = *req.DeliveryFee
	}
	if req.FreeDeliveryMin != nil {
		franchise.FreeDeliveryMin = *req.FreeDeliveryMin
	}

	if err := h.DB.Save(&franchise).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update franchise"})
		return
	}

	h.DB.Preload("StoreHours").First(&franchise, franchise.ID)
	c.JSON(http.StatusOK, franchise)
}

func (h *FranchiseHandler) GetMyProducts(c *gin.Context) {
	franchiseID, _ := c.Get("franchise_id")

	var fps []models.FranchiseProduct
	query := h.DB.Preload("Product").Preload("Product.Category").Preload("Product.Images").
		Where("franchise_id = ?", franchiseID)

	if search := c.Query("search"); search != "" {
		query = query.Joins("JOIN products ON products.id = franchise_products.product_id").
			Where("LOWER(products.item_name) LIKE LOWER(?)", "%"+search+"%")
	}

	if err := query.Find(&fps).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch products"})
		return
	}

	c.JSON(http.StatusOK, fps)
}

func (h *FranchiseHandler) UpdateProductStock(c *gin.Context) {
	franchiseID, _ := c.Get("franchise_id")
	productID := c.Param("id")

	var fp models.FranchiseProduct
	if err := h.DB.Where("franchise_id = ? AND product_id = ?", franchiseID, productID).First(&fp).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Franchise product not found"})
		return
	}

	var req struct {
		StockQuantity *int    `json:"stock_quantity"`
		ReorderLevel  *int    `json:"reorder_level"`
		ShelfLocation *string `json:"shelf_location"`
		IsAvailable   *bool   `json:"is_available"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": utils.SanitizeValidationError(err)})
		return
	}

	if req.StockQuantity != nil {
		fp.StockQuantity = *req.StockQuantity
	}
	if req.ReorderLevel != nil {
		fp.ReorderLevel = *req.ReorderLevel
	}
	if req.ShelfLocation != nil {
		fp.ShelfLocation = *req.ShelfLocation
	}
	if req.IsAvailable != nil {
		fp.IsAvailable = *req.IsAvailable
	}

	if err := h.DB.Save(&fp).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update stock"})
		return
	}

	h.DB.Preload("Product").Preload("Product.Category").Preload("Product.Images").First(&fp, fp.ID)
	c.JSON(http.StatusOK, fp)
}

func (h *FranchiseHandler) UpdateProductPricing(c *gin.Context) {
	franchiseID, _ := c.Get("franchise_id")
	productID := c.Param("id")

	var fp models.FranchiseProduct
	if err := h.DB.Where("franchise_id = ? AND product_id = ?", franchiseID, productID).First(&fp).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Franchise product not found"})
		return
	}

	var req struct {
		RetailPriceOverride    *float64   `json:"retail_price_override"`
		PromotionPriceOverride *float64   `json:"promotion_price_override"`
		PromotionStartOverride *time.Time `json:"promotion_start_override"`
		PromotionEndOverride   *time.Time `json:"promotion_end_override"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": utils.SanitizeValidationError(err)})
		return
	}

	fp.RetailPriceOverride = req.RetailPriceOverride
	fp.PromotionPriceOverride = req.PromotionPriceOverride
	fp.PromotionStartOverride = req.PromotionStartOverride
	fp.PromotionEndOverride = req.PromotionEndOverride

	if err := h.DB.Save(&fp).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update pricing"})
		return
	}

	h.DB.Preload("Product").First(&fp, fp.ID)
	c.JSON(http.StatusOK, fp)
}

func (h *FranchiseHandler) GetMyOrders(c *gin.Context) {
	franchiseID, _ := c.Get("franchise_id")

	var orders []models.Order
	query := h.DB.Preload("Items").Preload("Items.Product").Preload("Items.Product.Images").Preload("User").
		Where("franchise_id = ?", franchiseID)

	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Order("created_at DESC").Find(&orders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch orders"})
		return
	}

	c.JSON(http.StatusOK, orders)
}

func (h *FranchiseHandler) UpdateOrderStatus(c *gin.Context) {
	franchiseID, _ := c.Get("franchise_id")
	orderID := c.Param("id")

	var order models.Order
	if err := h.DB.Where("id = ? AND franchise_id = ?", orderID, franchiseID).First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	var req struct {
		Status models.OrderStatus `json:"status" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": utils.SanitizeValidationError(err)})
		return
	}

	// Validate state transition using the shared state machine
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
			var fp models.FranchiseProduct
			if err := h.DB.Where("franchise_id = ? AND product_id = ?", franchiseID, item.ProductID).First(&fp).Error; err == nil {
				fp.StockQuantity += item.Quantity
				h.DB.Save(&fp)
			} else {
				// Fallback to master product stock
				h.DB.Model(&models.Product{}).Where("id = ?", item.ProductID).
					UpdateColumn("stock_quantity", gorm.Expr("stock_quantity + ?", item.Quantity))
			}
		}
	}

	h.DB.Preload("Items").Preload("Items.Product").Preload("User").First(&order, order.ID)

	// Send status update email (non-blocking)
	if order.User.Email != "" {
		utils.SendOrderStatusUpdate(order.User.Email, order.User.Name, order.OrderNumber, string(req.Status))
	}

	c.JSON(http.StatusOK, order)
}

func (h *FranchiseHandler) GetMyStaff(c *gin.Context) {
	franchiseID, _ := c.Get("franchise_id")

	var staff []models.FranchiseStaff
	if err := h.DB.Preload("User").Where("franchise_id = ?", franchiseID).Find(&staff).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch staff"})
		return
	}

	c.JSON(http.StatusOK, staff)
}

func (h *FranchiseHandler) InviteStaff(c *gin.Context) {
	franchiseID, _ := c.Get("franchise_id")
	fID := franchiseID.(uuid.UUID)

	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Name     string `json:"name"`
		Password string `json:"password" binding:"required,min=8"`
		Role     string `json:"role"` // manager or staff
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": utils.SanitizeValidationError(err)})
		return
	}

	if req.Role == "" {
		req.Role = "staff"
	}
	if req.Role != "manager" && req.Role != "staff" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Role must be 'manager' or 'staff'"})
		return
	}

	tx := h.DB.Begin()

	// Create or find user
	var user models.User
	if err := tx.Where("email = ?", req.Email).First(&user).Error; err != nil {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
			return
		}

		user = models.User{
			ID:          uuid.New(),
			Email:       req.Email,
			Password:    string(hashedPassword),
			Name:        req.Name,
			Role:        "franchise_staff",
			FranchiseID: &fID,
		}

		if err := tx.Create(&user).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}
	} else {
		// Update existing user's franchise association
		tx.Model(&user).Updates(map[string]interface{}{
			"franchise_id": fID,
			"role":         "franchise_staff",
		})
	}

	// Create franchise staff record
	staff := models.FranchiseStaff{
		FranchiseID: fID,
		UserID:      user.ID,
		Role:        req.Role,
	}

	if err := tx.Create(&staff).Error; err != nil {
		tx.Rollback()
		if strings.Contains(err.Error(), "duplicate") {
			c.JSON(http.StatusConflict, gin.H{"error": "User is already staff at a franchise"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add staff"})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to complete operation"})
		return
	}

	h.DB.Preload("User").First(&staff, staff.ID)
	c.JSON(http.StatusCreated, staff)
}

func (h *FranchiseHandler) RemoveStaff(c *gin.Context) {
	franchiseID, _ := c.Get("franchise_id")
	staffID := c.Param("id")

	var staff models.FranchiseStaff
	if err := h.DB.Where("id = ? AND franchise_id = ?", staffID, franchiseID).First(&staff).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Staff member not found"})
		return
	}

	// Remove franchise association from user
	h.DB.Model(&models.User{}).Where("id = ?", staff.UserID).Updates(map[string]interface{}{
		"franchise_id": nil,
		"role":         "customer",
	})

	if err := h.DB.Delete(&staff).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove staff"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Staff member removed"})
}

func (h *FranchiseHandler) GetStoreHours(c *gin.Context) {
	franchiseID, _ := c.Get("franchise_id")

	var hours []models.StoreHours
	if err := h.DB.Where("franchise_id = ?", franchiseID).Order("day_of_week").Find(&hours).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch store hours"})
		return
	}

	c.JSON(http.StatusOK, hours)
}

func (h *FranchiseHandler) UpdateStoreHours(c *gin.Context) {
	franchiseID, _ := c.Get("franchise_id")

	var req []struct {
		DayOfWeek int    `json:"day_of_week" binding:"required"`
		OpenTime  string `json:"open_time" binding:"required"`
		CloseTime string `json:"close_time" binding:"required"`
		IsClosed  bool   `json:"is_closed"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": utils.SanitizeValidationError(err)})
		return
	}

	for _, h2 := range req {
		// Validate close_time > open_time when not closed
		if !h2.IsClosed && h2.CloseTime <= h2.OpenTime {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("Close time (%s) must be after open time (%s) for day %d", h2.CloseTime, h2.OpenTime, h2.DayOfWeek),
			})
			return
		}
		h.DB.Model(&models.StoreHours{}).
			Where("franchise_id = ? AND day_of_week = ?", franchiseID, h2.DayOfWeek).
			Updates(map[string]interface{}{
				"open_time":  h2.OpenTime,
				"close_time": h2.CloseTime,
				"is_closed":  h2.IsClosed,
			})
	}

	var hours []models.StoreHours
	h.DB.Where("franchise_id = ?", franchiseID).Order("day_of_week").Find(&hours)
	c.JSON(http.StatusOK, hours)
}

func (h *FranchiseHandler) GetMyPromotions(c *gin.Context) {
	franchiseID, _ := c.Get("franchise_id")

	var promotions []models.FranchisePromotion
	if err := h.DB.Where("franchise_id = ?", franchiseID).Order("created_at DESC").Find(&promotions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch promotions"})
		return
	}

	c.JSON(http.StatusOK, promotions)
}

func (h *FranchiseHandler) CreatePromotion(c *gin.Context) {
	franchiseID, _ := c.Get("franchise_id")
	fID := franchiseID.(uuid.UUID)

	var req struct {
		Title       string     `json:"title" binding:"required"`
		Description string     `json:"description"`
		Image       string     `json:"image"`
		ProductURL  string     `json:"product_url"`
		IsActive    bool       `json:"is_active"`
		StartDate   *time.Time `json:"start_date"`
		EndDate     *time.Time `json:"end_date"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": utils.SanitizeValidationError(err)})
		return
	}

	promo := models.FranchisePromotion{
		FranchiseID: fID,
		Title:       req.Title,
		Description: req.Description,
		Image:       req.Image,
		ProductURL:  req.ProductURL,
		IsActive:    req.IsActive,
		StartDate:   req.StartDate,
		EndDate:     req.EndDate,
	}

	if err := h.DB.Create(&promo).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create promotion"})
		return
	}

	c.JSON(http.StatusCreated, promo)
}

func (h *FranchiseHandler) UpdatePromotion(c *gin.Context) {
	franchiseID, _ := c.Get("franchise_id")
	promoID := c.Param("id")

	var promo models.FranchisePromotion
	if err := h.DB.Where("id = ? AND franchise_id = ?", promoID, franchiseID).First(&promo).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Promotion not found"})
		return
	}

	var req struct {
		Title       *string    `json:"title"`
		Description *string    `json:"description"`
		Image       *string    `json:"image"`
		ProductURL  *string    `json:"product_url"`
		IsActive    *bool      `json:"is_active"`
		StartDate   *time.Time `json:"start_date"`
		EndDate     *time.Time `json:"end_date"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": utils.SanitizeValidationError(err)})
		return
	}

	if req.Title != nil {
		promo.Title = *req.Title
	}
	if req.Description != nil {
		promo.Description = *req.Description
	}
	if req.Image != nil {
		promo.Image = *req.Image
	}
	if req.ProductURL != nil {
		promo.ProductURL = *req.ProductURL
	}
	if req.IsActive != nil {
		promo.IsActive = *req.IsActive
	}
	if req.StartDate != nil {
		promo.StartDate = req.StartDate
	}
	if req.EndDate != nil {
		promo.EndDate = req.EndDate
	}

	if err := h.DB.Save(&promo).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update promotion"})
		return
	}

	c.JSON(http.StatusOK, promo)
}

func (h *FranchiseHandler) DeletePromotion(c *gin.Context) {
	franchiseID, _ := c.Get("franchise_id")
	promoID := c.Param("id")

	var promo models.FranchisePromotion
	if err := h.DB.Where("id = ? AND franchise_id = ?", promoID, franchiseID).First(&promo).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Promotion not found"})
		return
	}

	if err := h.DB.Delete(&promo).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete promotion"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Promotion deleted"})
}

func (h *FranchiseHandler) GetDashboard(c *gin.Context) {
	franchiseID, _ := c.Get("franchise_id")

	// Revenue and order stats
	var totalRevenue float64
	var totalOrders int64
	var todayOrders int64
	today := time.Now().Truncate(24 * time.Hour)

	h.DB.Model(&models.Order{}).Where("franchise_id = ?", franchiseID).Count(&totalOrders)
	h.DB.Model(&models.Order{}).Where("franchise_id = ?", franchiseID).
		Select("COALESCE(SUM(total), 0)").Scan(&totalRevenue)
	h.DB.Model(&models.Order{}).Where("franchise_id = ? AND created_at >= ?", franchiseID, today).Count(&todayOrders)

	// Pending orders
	var pendingOrders int64
	h.DB.Model(&models.Order{}).Where("franchise_id = ? AND status IN ?", franchiseID,
		[]string{"pending", "confirmed", "preparing"}).Count(&pendingOrders)

	// Low stock alerts
	var lowStockCount int64
	h.DB.Model(&models.FranchiseProduct{}).
		Where("franchise_id = ? AND stock_quantity <= reorder_level AND is_available = ?", franchiseID, true).
		Count(&lowStockCount)

	// Recent orders
	var recentOrders []models.Order
	h.DB.Preload("Items").Preload("User").
		Where("franchise_id = ?", franchiseID).
		Order("created_at DESC").
		Limit(10).
		Find(&recentOrders)

	// Today's revenue
	var todayRevenue float64
	h.DB.Model(&models.Order{}).
		Where("franchise_id = ? AND created_at >= ?", franchiseID, today).
		Select("COALESCE(SUM(total), 0)").Scan(&todayRevenue)

	// Staff count
	var staffCount int64
	h.DB.Model(&models.FranchiseStaff{}).Where("franchise_id = ?", franchiseID).Count(&staffCount)

	// Product count
	var productCount int64
	h.DB.Model(&models.FranchiseProduct{}).Where("franchise_id = ?", franchiseID).Count(&productCount)

	c.JSON(http.StatusOK, gin.H{
		"total_revenue":    totalRevenue,
		"total_orders":     totalOrders,
		"today_orders":     todayOrders,
		"today_revenue":    todayRevenue,
		"pending_orders":   pendingOrders,
		"low_stock_alerts": lowStockCount,
		"staff_count":      staffCount,
		"product_count":    productCount,
		"recent_orders":    recentOrders,
	})
}

// CreateProduct allows franchise owners to create a new product linked to their franchise
func (h *FranchiseHandler) CreateProduct(c *gin.Context) {
	franchiseID, _ := c.Get("franchise_id")
	fID := franchiseID.(uuid.UUID)

	var req struct {
		ItemName         string   `json:"item_name" binding:"required"`
		ShortDescription string   `json:"short_description"`
		Brand            string   `json:"brand"`
		PackSize         string   `json:"pack_size"`
		CostPrice        float64  `json:"cost_price" binding:"required,min=0.01"`
		RetailPrice      float64  `json:"retail_price" binding:"required,min=0.01"`
		StockQuantity    int      `json:"stock_quantity"`
		ReorderLevel     int      `json:"reorder_level"`
		CategoryID       string   `json:"category_id" binding:"required"`
		ImageURL         string   `json:"image_url"`
		Status           string   `json:"status"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": utils.SanitizeValidationError(err)})
		return
	}

	categoryUUID, err := uuid.Parse(req.CategoryID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category ID"})
		return
	}

	// Validate category exists
	if err := h.DB.First(&models.Category{}, "id = ?", categoryUUID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category"})
		return
	}

	status := req.Status
	if status == "" {
		status = "active"
	}

	// Auto-generate SKU
	sku := fmt.Sprintf("FRN-%d%04d", time.Now().Unix()%100000, fID[0:2][0])

	product := models.Product{
		ID:               uuid.New(),
		SKU:              sku,
		ItemName:         req.ItemName,
		ShortDescription: req.ShortDescription,
		Brand:            req.Brand,
		PackSize:         req.PackSize,
		CostPrice:        req.CostPrice,
		RetailPrice:      req.RetailPrice,
		StockQuantity:    req.StockQuantity,
		ReorderLevel:     req.ReorderLevel,
		CategoryID:       categoryUUID,
		Status:           status,
	}

	tx := h.DB.Begin()

	if err := tx.Create(&product).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create product"})
		return
	}

	// Create image record if URL provided
	if req.ImageURL != "" {
		img := models.ProductImage{
			ProductID: product.ID,
			ImageURL:  req.ImageURL,
			IsPrimary: true,
		}
		tx.Create(&img)
	}

	// Create FranchiseProduct entry linking to this franchise
	fp := models.FranchiseProduct{
		FranchiseID:   fID,
		ProductID:     product.ID,
		StockQuantity: req.StockQuantity,
		ReorderLevel:  req.ReorderLevel,
		IsAvailable:   status == "active",
	}
	if err := tx.Create(&fp).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to link product to franchise"})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to complete operation"})
		return
	}

	tx.Preload("Category").Preload("Images").First(&product, product.ID)
	c.JSON(http.StatusCreated, product)
}
