package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"grabbi-backend/firebase"
	"grabbi-backend/models"
	"grabbi-backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type FranchiseHandler struct {
	DB      *gorm.DB
	Storage firebase.StorageClient
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

// StoreHoursResponse represents store hours for API response
type StoreHoursResponse struct {
	DayOfWeek int    `json:"day_of_week"` // 0=Sunday, 6=Saturday
	OpenTime  string `json:"open_time"`
	CloseTime string `json:"close_time"`
	IsClosed  bool   `json:"is_closed"`
}

// StoreStatusResponse represents the current store open/closed status
type StoreStatusResponse struct {
	IsOpen       bool   `json:"is_open"`
	CurrentDay   int    `json:"current_day"`   // 0=Sunday, 6=Saturday
	OpenTime     string `json:"open_time"`     // Today's open time
	CloseTime    string `json:"close_time"`    // Today's close time
	Message      string `json:"message"`       // Human-readable status message
	NextOpenDay  *int   `json:"next_open_day,omitempty"`  // Next day store is open (if currently closed)
	NextOpenTime *string `json:"next_open_time,omitempty"` // Next open time
}

// FranchiseWithDistance represents a franchise with calculated distance and delivery time
type FranchiseWithDistance struct {
	ID              uuid.UUID            `json:"id"`
	Name            string               `json:"name"`
	Slug            string               `json:"slug"`
	Address         string               `json:"address"`
	City            string               `json:"city"`
	PostCode        string               `json:"post_code"`
	Latitude        float64              `json:"latitude"`
	Longitude       float64              `json:"longitude"`
	DeliveryRadius  float64              `json:"delivery_radius"`
	DeliveryFee     float64              `json:"delivery_fee"`
	FreeDeliveryMin float64              `json:"free_delivery_min"`
	Phone           string               `json:"phone"`
	Email           string               `json:"email"`
	IsActive        bool                 `json:"is_active"`
	Distance        float64              `json:"distance"`
	DeliveryTime    string               `json:"delivery_time"`
	StoreHours      []StoreHoursResponse `json:"store_hours"`
	StoreStatus     StoreStatusResponse  `json:"store_status"`
}

// estimateDeliveryTime calculates estimated delivery time based on distance in km
func estimateDeliveryTime(distanceKM float64) string {
	// Base time: 15 minutes + additional time based on distance
	// Assume average delivery speed of 20 km/h in urban areas
	baseMinutes := 15
	travelMinutes := int((distanceKM / 20.0) * 60)

	minTime := baseMinutes + travelMinutes
	maxTime := minTime + 15 // Add 15 min buffer

	return fmt.Sprintf("%d-%d min", minTime, maxTime)
}

// dayNames maps day of week number to name
var dayNames = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

// calculateStoreStatus determines if a store is currently open based on store hours
func calculateStoreStatus(hours []models.StoreHours) StoreStatusResponse {
	now := time.Now()
	currentDay := int(now.Weekday()) // 0=Sunday, 6=Saturday
	currentTime := now.Format("15:04")

	// Build a map of hours for quick lookup
	hoursMap := make(map[int]models.StoreHours)
	for _, h := range hours {
		hoursMap[h.DayOfWeek] = h
	}

	// Get today's hours
	todayHours, exists := hoursMap[currentDay]
	if !exists || todayHours.IsClosed {
		// Store is closed today, find next open day
		return findNextOpenDay(hoursMap, currentDay)
	}

	// Check if current time is within open hours
	if currentTime >= todayHours.OpenTime && currentTime <= todayHours.CloseTime {
		return StoreStatusResponse{
			IsOpen:    true,
			CurrentDay: currentDay,
			OpenTime:  todayHours.OpenTime,
			CloseTime: todayHours.CloseTime,
			Message:   fmt.Sprintf("Open until %s", formatTime(todayHours.CloseTime)),
		}
	}

	// Check if store hasn't opened yet today
	if currentTime < todayHours.OpenTime {
		return StoreStatusResponse{
			IsOpen:    false,
			CurrentDay: currentDay,
			OpenTime:  todayHours.OpenTime,
			CloseTime: todayHours.CloseTime,
			Message:   fmt.Sprintf("Opens today at %s", formatTime(todayHours.OpenTime)),
		}
	}

	// Store has closed for today, find next open day
	return findNextOpenDay(hoursMap, currentDay)
}

// findNextOpenDay finds the next day the store is open
func findNextOpenDay(hoursMap map[int]models.StoreHours, currentDay int) StoreStatusResponse {
	// Look for up to 7 days ahead
	for i := 1; i <= 7; i++ {
		nextDay := (currentDay + i) % 7
		if nextHours, exists := hoursMap[nextDay]; exists && !nextHours.IsClosed {
			nextDayPtr := &nextDay
			return StoreStatusResponse{
				IsOpen:       false,
				CurrentDay:   currentDay,
				Message:      fmt.Sprintf("Closed · Opens %s at %s", dayNames[nextDay], formatTime(nextHours.OpenTime)),
				NextOpenDay:  nextDayPtr,
				NextOpenTime: &nextHours.OpenTime,
			}
		}
	}

	// No open days found (permanently closed?)
	return StoreStatusResponse{
		IsOpen:     false,
		CurrentDay: currentDay,
		Message:    "Temporarily closed",
	}
}

// formatTime converts 24-hour time to 12-hour format for display
func formatTime(time24 string) string {
	parts := strings.Split(time24, ":")
	if len(parts) != 2 {
		return time24
	}
	
	hour, _ := strconv.Atoi(parts[0])
	minute := parts[1]
	
	var period string
	if hour >= 12 {
		period = "PM"
		if hour > 12 {
			hour -= 12
		}
	} else {
		period = "AM"
		if hour == 0 {
			hour = 12
		}
	}
	
	return fmt.Sprintf("%d:%s %s", hour, minute, period)
}

// convertStoreHours converts models.StoreHours to StoreHoursResponse
func convertStoreHours(hours []models.StoreHours) []StoreHoursResponse {
	result := make([]StoreHoursResponse, len(hours))
	for i, h := range hours {
		result[i] = StoreHoursResponse{
			DayOfWeek: h.DayOfWeek,
			OpenTime:  h.OpenTime,
			CloseTime: h.CloseTime,
			IsClosed:  h.IsClosed,
		}
	}
	return result
}

// GetNearbyFranchises returns all active franchises within their delivery radius of the user's location
func (h *FranchiseHandler) GetNearbyFranchises(c *gin.Context) {
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

	// Validate latitude and longitude ranges
	if lat < -90 || lat > 90 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Latitude must be between -90 and 90"})
		return
	}
	if lng < -180 || lng > 180 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Longitude must be between -180 and 180"})
		return
	}

	var franchises []models.Franchise
	if err := h.DB.Preload("StoreHours").Where("is_active = ?", true).Find(&franchises).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch franchises"})
		return
	}

	var result []FranchiseWithDistance
	for _, f := range franchises {
		// Calculate distance using Haversine formula in kilometers
		distance := utils.HaversineKM(lat, lng, f.Latitude, f.Longitude)

		// Round distance to 1 decimal place for cleaner display
		distance = math.Round(distance*10) / 10

		// Only include franchises where user is within the franchise's delivery radius
		if distance <= f.DeliveryRadius {
			// Convert store hours and calculate current status
			storeHours := convertStoreHours(f.StoreHours)
			storeStatus := calculateStoreStatus(f.StoreHours)

			result = append(result, FranchiseWithDistance{
				ID:              f.ID,
				Name:            f.Name,
				Slug:            f.Slug,
				Address:         f.Address,
				City:            f.City,
				PostCode:        f.PostCode,
				Latitude:        f.Latitude,
				Longitude:       f.Longitude,
				DeliveryRadius:  f.DeliveryRadius,
				DeliveryFee:     f.DeliveryFee,
				FreeDeliveryMin: f.FreeDeliveryMin,
				Phone:           f.Phone,
				Email:           f.Email,
				IsActive:        f.IsActive,
				Distance:        distance,
				DeliveryTime:    estimateDeliveryTime(distance),
				StoreHours:      storeHours,
				StoreStatus:     storeStatus,
			})
		}
	}

	// Sort results by distance (nearest first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Distance < result[j].Distance
	})

	c.JSON(http.StatusOK, gin.H{"franchises": result})
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
	// Use INNER JOIN to ensure we only get FranchiseProducts where the Product exists and is not soft-deleted
	// IMPORTANT: Only preload NON-DELETED images
	var fps []models.FranchiseProduct
	query := h.DB.InnerJoins("Product").Preload("Product.Category").Preload("Product.Images", "deleted_at IS NULL").
		Where("franchise_id = ? AND is_available = ?", franchiseID, true)

	if search := c.Query("search"); search != "" {
		query = query.Where("LOWER(Product.item_name) LIKE LOWER(?)", "%"+search+"%")
	}

	if categoryID := c.Query("category_id"); categoryID != "" {
		query = query.Where("Product.category_id = ?", categoryID)
	}

	if err := query.Find(&fps).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch products"})
		return
	}

	// Merge overrides onto master product data
	type MergedProduct struct {
		models.Product
		FranchiseStock     int      `json:"franchise_stock"`
		FranchisePrice     float64  `json:"franchise_price"`
		PromoPrice         *float64 `json:"franchise_promo_price,omitempty"`
		ShelfLocation      string   `json:"franchise_shelf_location"`
		FranchiseAvailable bool     `json:"franchise_available"`
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

// parseFloat64Ptr converts an interface{} to *float64, handling empty strings as nil
func parseFloat64Ptr(val interface{}) *float64 {
	switch v := val.(type) {
	case float64:
		return &v
	case string:
		if v == "" {
			return nil
		}
		// Try to parse the string as float
		var f float64
		if _, err := fmt.Sscanf(v, "%f", &f); err == nil {
			return &f
		}
		return nil
	case nil:
		return nil
	default:
		return nil
	}
}

func (h *FranchiseHandler) CreateFranchise(c *gin.Context) {
	// Use map to accept raw JSON, then parse manually to handle empty strings
	var rawReq map[string]interface{}
	if err := c.ShouldBindJSON(&rawReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": utils.SanitizeValidationError(err)})
		return
	}

	// Validate required fields
	name, _ := rawReq["name"].(string)
	slug, _ := rawReq["slug"].(string)
	ownerEmail, _ := rawReq["owner_email"].(string)
	ownerPassword, _ := rawReq["owner_password"].(string)

	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug is required"})
		return
	}
	if ownerEmail == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "owner_email is required"})
		return
	}
	if len(ownerPassword) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "owner_password must be at least 8 characters"})
		return
	}

	// Extract values from raw request
	ownerName, _ := rawReq["owner_name"].(string)
	address, _ := rawReq["address"].(string)
	city, _ := rawReq["city"].(string)
	postCode, _ := rawReq["post_code"].(string)
	phone, _ := rawReq["phone"].(string)
	email, _ := rawReq["email"].(string)

	// Parse latitude and longitude
	var latitude, longitude float64
	if lat, ok := rawReq["latitude"].(float64); ok {
		latitude = lat
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "latitude is required and must be a number"})
		return
	}
	if lng, ok := rawReq["longitude"].(float64); ok {
		longitude = lng
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "longitude is required and must be a number"})
		return
	}

	// Parse optional numeric fields
	var deliveryRadius float64
	if dr, ok := rawReq["delivery_radius"].(float64); ok {
		deliveryRadius = dr
	}

	deliveryFee := parseFloat64Ptr(rawReq["delivery_fee"])
	freeDeliveryMin := parseFloat64Ptr(rawReq["free_delivery_min"])

	var deliveryFeeVal float64
	if deliveryFee != nil {
		deliveryFeeVal = *deliveryFee
	}

	var freeDeliveryMinVal float64
	if freeDeliveryMin != nil {
		freeDeliveryMinVal = *freeDeliveryMin
	}

	tx := h.DB.Begin()

	// Create or find the owner user (including soft-deleted users to avoid unique constraint violation)
	var owner models.User
	if err := tx.Unscoped().Where("email = ?", ownerEmail).First(&owner).Error; err != nil {
		// No user found at all — create new user as franchise owner
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(ownerPassword), bcrypt.DefaultCost)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
			return
		}

		owner = models.User{
			ID:       uuid.New(),
			Email:    ownerEmail,
			Password: string(hashedPassword),
			Name:     ownerName,
			Role:     "franchise_owner",
		}

		if err := tx.Create(&owner).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create owner user"})
			return
		}
	} else if owner.DeletedAt.Valid {
		// Restore soft-deleted user and update their details
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(ownerPassword), bcrypt.DefaultCost)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
			return
		}
		if err := tx.Unscoped().Model(&owner).Updates(map[string]interface{}{
			"deleted_at": nil,
			"role":       "franchise_owner",
			"name":       ownerName,
			"password":   string(hashedPassword),
		}).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to restore owner user"})
			return
		}
	}
	// else: existing active user found, reuse as-is

	franchise := models.Franchise{
		Name:            name,
		Slug:            slug,
		OwnerID:         owner.ID,
		Address:         address,
		City:            city,
		PostCode:        postCode,
		Latitude:        latitude,
		Longitude:       longitude,
		DeliveryRadius:  deliveryRadius,
		DeliveryFee:     deliveryFeeVal,
		FreeDeliveryMin: freeDeliveryMinVal,
		Phone:           phone,
		Email:           email,
		IsActive:        true,
	}

	if franchise.DeliveryRadius == 0 {
		franchise.DeliveryRadius = 5
	}

	if err := tx.Create(&franchise).Error; err != nil {
		tx.Rollback()
		if strings.Contains(err.Error(), "idx_franchises_slug") || strings.Contains(err.Error(), "duplicate key") {
			c.JSON(http.StatusConflict, gin.H{"error": "A franchise with this slug already exists"})
			return
		}
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
			"error":         "Cannot delete franchise with existing dependencies",
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
		Latitude        *float64 `json:"latitude"`
		Longitude       *float64 `json:"longitude"`
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
	if req.Latitude != nil {
		if *req.Latitude < -90 || *req.Latitude > 90 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Latitude must be between -90 and 90"})
			return
		}
		franchise.Latitude = *req.Latitude
	}
	if req.Longitude != nil {
		if *req.Longitude < -180 || *req.Longitude > 180 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Longitude must be between -180 and 180"})
			return
		}
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

	if err := h.DB.Save(&franchise).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update franchise"})
		return
	}

	h.DB.Preload("StoreHours").First(&franchise, franchise.ID)
	c.JSON(http.StatusOK, franchise)
}

func (h *FranchiseHandler) GetMyProducts(c *gin.Context) {
	franchiseID, _ := c.Get("franchise_id")

	// First, get franchise products (excluding franchise-deleted ones)
	var fps []models.FranchiseProduct
	query := h.DB.Where("franchise_id = ?", franchiseID).
		Where("franchise_products.deleted_at IS NULL") // Exclude franchise-deleted products

	if search := c.Query("search"); search != "" {
		query = query.Joins("JOIN products ON products.id = franchise_products.product_id").
			Where("LOWER(products.item_name) LIKE LOWER(?)", "%"+search+"%")
	}

	if err := query.Find(&fps).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch products"})
		return
	}

	// Get product IDs
	productIDs := make([]uuid.UUID, len(fps))
	for i, fp := range fps {
		productIDs[i] = fp.ProductID
	}

	// Fetch ALL products (including soft-deleted) using Unscoped
	// But for Images, only fetch non-deleted ones
	var products []models.Product
	if len(productIDs) > 0 {
		if err := h.DB.Unscoped().Preload("Category").
			Preload("Images", "deleted_at IS NULL").
			Where("id IN ?", productIDs).
			Find(&products).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch product details"})
			return
		}
	}

	// Create a map for quick product lookup
	productMap := make(map[uuid.UUID]models.Product)
	for _, p := range products {
		productMap[p.ID] = p
	}

	// Add is_delisted_by_admin flag to response
	type FranchiseProductWithDelisted struct {
		models.FranchiseProduct
		IsDelistedByAdmin bool           `json:"is_delisted_by_admin"`
		Product           models.Product `json:"product"`
	}

	var result []FranchiseProductWithDelisted
	for _, fp := range fps {
		product := productMap[fp.ProductID]
		result = append(result, FranchiseProductWithDelisted{
			FranchiseProduct:  fp,
			IsDelistedByAdmin: product.DeletedAt.Valid, // Product is delisted if admin soft-deleted it
			Product:           product,
		})
	}

	c.JSON(http.StatusOK, result)
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

	h.DB.Preload("Product").Preload("Product.Category").Preload("Product.Images", "deleted_at IS NULL").First(&fp, fp.ID)
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
	query := h.DB.Preload("Items").Preload("Items.Product").Preload("Items.Product.Images", "deleted_at IS NULL").Preload("User").
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

	// Get franchise details for email
	var franchise models.Franchise
	if err := h.DB.Where("id = ?", fID).First(&franchise).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Franchise not found"})
		return
	}

	tx := h.DB.Begin()

	// Create or find user
	var user models.User
	var isNewUser bool
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
		isNewUser = true
	} else {
		// Update existing user's franchise association and name
		updateData := map[string]interface{}{
			"franchise_id": fID,
			"role":         "franchise_staff",
		}
		// Update name if provided in the request
		if req.Name != "" {
			updateData["name"] = req.Name
		}
		tx.Model(&user).Updates(updateData)
		isNewUser = false
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

	// Send staff invitation email
	franchiseURL := os.Getenv("FRANCHISE_URL")
	if franchiseURL == "" {
		franchiseURL = "http://localhost:5175" // default fallback
	}
	
	// Only send email with password for new users
	if isNewUser {
		utils.SendStaffInvitationEmail(req.Email, req.Name, franchise.Name, req.Role, req.Password, franchiseURL)
	} else {
		// For existing users, send a simpler notification without password
		utils.SendStaffAddedEmail(req.Email, user.Name, franchise.Name, req.Role, franchiseURL)
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

	// Define the hour entry structure
	type HourEntry struct {
		DayOfWeek int    `json:"day_of_week"`
		OpenTime  string `json:"open_time"`
		CloseTime string `json:"close_time"`
		IsClosed  bool   `json:"is_closed"`
	}

	// Get raw body bytes - this caches the body in the context
	rawBody, err := c.GetRawData()
	if err != nil {
		log.Printf("UpdateStoreHours: Failed to read request body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	log.Printf("UpdateStoreHours: Raw body: %s", string(rawBody))

	// Try to parse as a direct array first
	var req []HourEntry
	if err := json.Unmarshal(rawBody, &req); err != nil {
		log.Printf("UpdateStoreHours: Direct array parsing failed: %v", err)

		// Try wrapped format: {"hours": [...]}
		var wrappedReq struct {
			Hours []HourEntry `json:"hours"`
		}
		if err2 := json.Unmarshal(rawBody, &wrappedReq); err2 != nil {
			log.Printf("UpdateStoreHours: Wrapped format parsing also failed: %v", err2)
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid JSON format. Send array [{day_of_week, open_time, close_time, is_closed}] or {hours: [...]}",
			})
			return
		}
		req = wrappedReq.Hours
		log.Printf("UpdateStoreHours: Successfully parsed wrapped format with %d entries", len(req))
	}

	log.Printf("UpdateStoreHours: Received %d entries for franchise %v", len(req), franchiseID)

	for i, h2 := range req {
		log.Printf("UpdateStoreHours: Entry %d - day_of_week=%d, open_time=%s, close_time=%s, is_closed=%v",
			i, h2.DayOfWeek, h2.OpenTime, h2.CloseTime, h2.IsClosed)

		// Validate day_of_week is in valid range
		if h2.DayOfWeek < 0 || h2.DayOfWeek > 6 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("Invalid day_of_week: %d (must be 0-6)", h2.DayOfWeek),
			})
			return
		}

		// Validate close_time > open_time when not closed
		if !h2.IsClosed && h2.CloseTime != "" && h2.OpenTime != "" && h2.CloseTime <= h2.OpenTime {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("Close time (%s) must be after open time (%s) for day %d", h2.CloseTime, h2.OpenTime, h2.DayOfWeek),
			})
			return
		}

		result := h.DB.Model(&models.StoreHours{}).
			Where("franchise_id = ? AND day_of_week = ?", franchiseID, h2.DayOfWeek).
			Updates(map[string]interface{}{
				"open_time":  h2.OpenTime,
				"close_time": h2.CloseTime,
				"is_closed":  h2.IsClosed,
			})

		if result.Error != nil {
			log.Printf("UpdateStoreHours: Error updating day %d: %v", h2.DayOfWeek, result.Error)
		} else {
			log.Printf("UpdateStoreHours: Updated day %d, rows affected: %d", h2.DayOfWeek, result.RowsAffected)
		}
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

	// Parse form data
	title := c.PostForm("title")
	if title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title is required"})
		return
	}

	description := c.PostForm("description")
	productURL := c.PostForm("product_url")
	isActive := c.PostForm("is_active") == "true"

	// Parse start_date and end_date
	var startDate, endDate *time.Time
	if startDateStr := c.PostForm("start_date"); startDateStr != "" {
		if parsedTime, err := time.Parse(time.RFC3339, startDateStr); err == nil {
			startDate = &parsedTime
		} else if parsedTime, err := time.Parse("2006-01-02", startDateStr); err == nil {
			startDate = &parsedTime
		}
	}
	if endDateStr := c.PostForm("end_date"); endDateStr != "" {
		if parsedTime, err := time.Parse(time.RFC3339, endDateStr); err == nil {
			endDate = &parsedTime
		} else if parsedTime, err := time.Parse("2006-01-02", endDateStr); err == nil {
			endDate = &parsedTime
		}
	}

	// Handle image upload
	var imageURL string
	fileHeader, err := c.FormFile("image")
	if err == nil {
		// Validate file upload (content type + size)
		if err := utils.ValidateFileUpload(fileHeader); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		file, err := fileHeader.Open()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to open uploaded file"})
			return
		}
		defer file.Close()

		imageURL, err = h.Storage.UploadPromotionImage(
			file,
			fileHeader.Filename,
			fileHeader.Header.Get("Content-Type"),
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Image upload failed"})
			return
		}
	}

	promo := models.FranchisePromotion{
		FranchiseID: fID,
		Title:       title,
		Description: description,
		Image:       imageURL,
		ProductURL:  productURL,
		IsActive:    isActive,
		StartDate:   startDate,
		EndDate:     endDate,
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

	// Parse form data
	if title := c.PostForm("title"); title != "" {
		promo.Title = title
	}
	if description := c.PostForm("description"); description != "" {
		promo.Description = description
	}
	if productURL := c.PostForm("product_url"); productURL != "" {
		promo.ProductURL = productURL
	}
	promo.IsActive = c.PostForm("is_active") == "true"

	// Parse start_date and end_date
	if startDateStr := c.PostForm("start_date"); startDateStr != "" {
		if parsedTime, err := time.Parse(time.RFC3339, startDateStr); err == nil {
			promo.StartDate = &parsedTime
		} else if parsedTime, err := time.Parse("2006-01-02", startDateStr); err == nil {
			promo.StartDate = &parsedTime
		}
	}
	if endDateStr := c.PostForm("end_date"); endDateStr != "" {
		if parsedTime, err := time.Parse(time.RFC3339, endDateStr); err == nil {
			promo.EndDate = &parsedTime
		} else if parsedTime, err := time.Parse("2006-01-02", endDateStr); err == nil {
			promo.EndDate = &parsedTime
		}
	}

	// Handle image update
	fileHeader, err := c.FormFile("image")
	if err == nil {
		// Validate file upload (content type + size)
		if err := utils.ValidateFileUpload(fileHeader); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Delete old image from Firebase if exists
		if promo.Image != "" {
			objectPath, pathErr := utils.ExtractObjectPath(promo.Image)
			if pathErr == nil {
				_ = h.Storage.DeleteFile(objectPath)
			}
		}

		// Upload new image
		file, openErr := fileHeader.Open()
		if openErr != nil {
			log.Printf("Failed to open uploaded file: %v", openErr)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to open uploaded file"})
			return
		}
		defer file.Close()

		imageURL, uploadErr := h.Storage.UploadPromotionImage(
			file,
			fileHeader.Filename,
			fileHeader.Header.Get("Content-Type"),
		)
		if uploadErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Image upload failed"})
			return
		}

		promo.Image = imageURL
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

	// Delete image from Firebase Storage if exists
	if promo.Image != "" && h.Storage != nil {
		objectPath, err := utils.ExtractObjectPath(promo.Image)
		if err == nil && objectPath != "" {
			if err := h.Storage.DeleteFile(objectPath); err != nil {
				log.Printf("Failed to delete promotion image from Firebase: %v", err)
			} else {
				log.Printf("Deleted promotion image from Firebase: %s", promo.Image)
			}
		}
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
// Supports multipart/form-data for image uploads
func (h *FranchiseHandler) CreateProduct(c *gin.Context) {
	franchiseID, _ := c.Get("franchise_id")
	fID := franchiseID.(uuid.UUID)

	// Validate required field: item_name
	itemName := c.PostForm("item_name")
	if itemName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "item_name is required"})
		return
	}

	// Validate required field: cost_price
	costPriceStr := c.PostForm("cost_price")
	if costPriceStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cost_price is required"})
		return
	}
	costPrice, err := strconv.ParseFloat(costPriceStr, 64)
	if err != nil || costPrice <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cost_price must be a positive number"})
		return
	}

	// Validate required field: retail_price
	retailPriceStr := c.PostForm("retail_price")
	if retailPriceStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "retail_price is required"})
		return
	}
	retailPrice, err := strconv.ParseFloat(retailPriceStr, 64)
	if err != nil || retailPrice <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "retail_price must be a positive number"})
		return
	}

	// Validate required field: category_id
	categoryIDStr := c.PostForm("category_id")
	if categoryIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "category_id is required"})
		return
	}
	categoryUUID, err := uuid.Parse(categoryIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category ID"})
		return
	}

	// Validate category exists
	if err := h.DB.First(&models.Category{}, "id = ?", categoryUUID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category"})
		return
	}

	status := c.PostForm("status")
	if status == "" {
		status = "active"
	}

	// Auto-generate SKU
	sku := fmt.Sprintf("FRN-%d%04d", time.Now().Unix()%100000, fID[0:2][0])

	// Build product from form data
	product := models.Product{
		ID:               uuid.New(),
		SKU:              sku,
		ItemName:         itemName,
		ShortDescription: c.PostForm("short_description"),
		LongDescription:  c.PostForm("long_description"),
		Brand:            c.PostForm("brand"),
		PackSize:         c.PostForm("pack_size"),
		CostPrice:        costPrice,
		RetailPrice:      retailPrice,
		StockQuantity:    parseFormInt(c.PostForm("stock_quantity")),
		ReorderLevel:     parseFormInt(c.PostForm("reorder_level")),
		ShelfLocation:    c.PostForm("shelf_location"),
		CategoryID:       categoryUUID,
		Status:           status,
	}

	// Optional numeric fields
	product.GrossMargin, _ = strconv.ParseFloat(c.PostForm("gross_margin"), 64)
	product.StaffDiscount, _ = strconv.ParseFloat(c.PostForm("staff_discount"), 64)
	product.TaxRate, _ = strconv.ParseFloat(c.PostForm("tax_rate"), 64)
	product.WeightVolume, _ = strconv.ParseFloat(c.PostForm("weight_volume"), 64)

	// Product identifiers
	product.BatchNumber = c.PostForm("batch_number")
	if barcode := c.PostForm("barcode"); barcode != "" {
		product.Barcode = &barcode
	}

	// Supplier info
	product.Supplier = c.PostForm("supplier")
	product.CountryOfOrigin = c.PostForm("country_of_origin")

	// Dietary flags
	product.IsGlutenFree = c.PostForm("is_gluten_free") == "true"
	product.IsVegetarian = c.PostForm("is_vegetarian") == "true"
	product.IsVegan = c.PostForm("is_vegan") == "true"
	product.IsAgeRestricted = c.PostForm("is_age_restricted") == "true"

	// Additional info
	product.AllergenInfo = c.PostForm("allergen_info")
	product.StorageType = c.PostForm("storage_type")
	product.IsOwnBrand = c.PostForm("is_own_brand") == "true"
	product.OnlineVisible = c.PostForm("online_visible") != "false" // Default true
	product.Notes = c.PostForm("notes")

	tx := h.DB.Begin()

	if err := tx.Create(&product).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create product"})
		return
	}

	// Handle image uploads
	form, err := c.MultipartForm()
	if err == nil && form != nil {
		files := form.File["images"]
		if len(files) > 0 && h.Storage != nil {
			var productImages []models.ProductImage
			for i, fileHeader := range files {
				// Validate file upload
				if err := utils.ValidateFileUpload(fileHeader); err != nil {
					log.Printf("Skipping invalid image: %v", err)
					continue
				}

				file, err := fileHeader.Open()
				if err != nil {
					log.Printf("Failed to open uploaded file: %v", err)
					continue
				}

				imageURL, err := h.Storage.UploadProductImage(
					file,
					fileHeader.Filename,
					fileHeader.Header.Get("Content-Type"),
				)
				file.Close()

				if err != nil {
					log.Printf("Failed to upload image: %v", err)
					continue
				}

				// First image is marked as primary
				productImage := models.ProductImage{
					ProductID: product.ID,
					ImageURL:  imageURL,
					IsPrimary: i == 0,
				}
				productImages = append(productImages, productImage)
			}

			if len(productImages) > 0 {
				if err := tx.Create(&productImages).Error; err != nil {
					log.Printf("Failed to save product images: %v", err)
				}
			}
		}
	}

	// Create FranchiseProduct entry linking to this franchise
	fp := models.FranchiseProduct{
		FranchiseID:   fID,
		ProductID:     product.ID,
		StockQuantity: product.StockQuantity,
		ReorderLevel:  product.ReorderLevel,
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

	h.DB.Preload("Category").Preload("Images").First(&product, product.ID)
	c.JSON(http.StatusCreated, product)
}

// parseFormInt parses a string to int, returning 0 if empty or invalid
func parseFormInt(val string) int {
	if val == "" {
		return 0
	}
	result, _ := strconv.Atoi(val)
	return result
}

// getFormKeys returns the keys from form value map for debugging
func getFormKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// getFileKeys returns the keys from file map for debugging
func getFileKeys(m map[string][]*multipart.FileHeader) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// UpdateProduct allows franchise owners to update an existing product
// Supports multipart/form-data for image uploads
// If the product was admin-deleted (delisted), updating it will restore it
func (h *FranchiseHandler) UpdateProduct(c *gin.Context) {
	franchiseID, _ := c.Get("franchise_id")
	productID := c.Param("id")

	// Find the FranchiseProduct by its ID (the ID passed is the FranchiseProduct.ID)
	var fp models.FranchiseProduct
	if err := h.DB.Where("id = ? AND franchise_id = ?", productID, franchiseID).First(&fp).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found in your franchise"})
		return
	}

	// Get the actual Product (use Unscoped to include admin-soft-deleted products)
	// IMPORTANT: Only preload NON-DELETED images to avoid GORM trying to re-save soft-deleted images
	var product models.Product
	if err := h.DB.Unscoped().Preload("Images", "deleted_at IS NULL").First(&product, "id = ?", fp.ProductID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	// Check if product was admin-deleted (delisted) - we'll restore it on update
	wasDelistedByAdmin := product.DeletedAt.Valid

	// Update product fields from form data
	if itemName := c.PostForm("item_name"); itemName != "" {
		product.ItemName = itemName
	}
	if shortDesc := c.PostForm("short_description"); shortDesc != "" {
		product.ShortDescription = shortDesc
	}
	if longDesc := c.PostForm("long_description"); longDesc != "" {
		product.LongDescription = longDesc
	}

	// Pricing
	if costPriceStr := c.PostForm("cost_price"); costPriceStr != "" {
		if costPrice, err := strconv.ParseFloat(costPriceStr, 64); err == nil && costPrice > 0 {
			product.CostPrice = costPrice
		}
	}
	if retailPriceStr := c.PostForm("retail_price"); retailPriceStr != "" {
		if retailPrice, err := strconv.ParseFloat(retailPriceStr, 64); err == nil && retailPrice > 0 {
			product.RetailPrice = retailPrice
		}
	}

	// Promotion pricing
	if promotionPriceStr := c.PostForm("promotion_price"); promotionPriceStr != "" {
		if promotionPrice, err := strconv.ParseFloat(promotionPriceStr, 64); err == nil {
			product.PromotionPrice = &promotionPrice
		}
	} else {
		// Clear promotion price if empty string sent
		product.PromotionPrice = nil
	}

	if promotionStartStr := c.PostForm("promotion_start"); promotionStartStr != "" {
		if promotionStart, err := time.Parse("2006-01-02", promotionStartStr); err == nil {
			product.PromotionStart = &promotionStart
		}
	} else {
		product.PromotionStart = nil
	}

	if promotionEndStr := c.PostForm("promotion_end"); promotionEndStr != "" {
		if promotionEnd, err := time.Parse("2006-01-02", promotionEndStr); err == nil {
			product.PromotionEnd = &promotionEnd
		}
	} else {
		product.PromotionEnd = nil
	}

	// Expiry date
	if expiryDateStr := c.PostForm("expiry_date"); expiryDateStr != "" {
		if expiryDate, err := time.Parse("2006-01-02", expiryDateStr); err == nil {
			product.ExpiryDate = &expiryDate
		}
	} else {
		product.ExpiryDate = nil
	}

	// Margin and Tax
	if grossMarginStr := c.PostForm("gross_margin"); grossMarginStr != "" {
		if grossMargin, err := strconv.ParseFloat(grossMarginStr, 64); err == nil {
			product.GrossMargin = grossMargin
		}
	}
	if staffDiscountStr := c.PostForm("staff_discount"); staffDiscountStr != "" {
		if staffDiscount, err := strconv.ParseFloat(staffDiscountStr, 64); err == nil {
			product.StaffDiscount = staffDiscount
		}
	}
	if taxRateStr := c.PostForm("tax_rate"); taxRateStr != "" {
		if taxRate, err := strconv.ParseFloat(taxRateStr, 64); err == nil {
			product.TaxRate = taxRate
		}
	}

	// Product Identifiers
	if batchNumber := c.PostForm("batch_number"); batchNumber != "" {
		product.BatchNumber = batchNumber
	}
	if barcode := c.PostForm("barcode"); barcode != "" {
		product.Barcode = &barcode
	}

	// Inventory
	if stockQtyStr := c.PostForm("stock_quantity"); stockQtyStr != "" {
		if stockQty, err := strconv.Atoi(stockQtyStr); err == nil {
			product.StockQuantity = stockQty
			fp.StockQuantity = stockQty
		}
	}
	if reorderLevelStr := c.PostForm("reorder_level"); reorderLevelStr != "" {
		if reorderLevel, err := strconv.Atoi(reorderLevelStr); err == nil {
			product.ReorderLevel = reorderLevel
			fp.ReorderLevel = reorderLevel
		}
	}
	if shelfLocation := c.PostForm("shelf_location"); shelfLocation != "" {
		product.ShelfLocation = shelfLocation
		fp.ShelfLocation = shelfLocation
	}

	// Weight/Volume
	if weightVolumeStr := c.PostForm("weight_volume"); weightVolumeStr != "" {
		if weightVolume, err := strconv.ParseFloat(weightVolumeStr, 64); err == nil {
			product.WeightVolume = weightVolume
		}
	}

	// Supplier info
	if supplier := c.PostForm("supplier"); supplier != "" {
		product.Supplier = supplier
	}
	if countryOfOrigin := c.PostForm("country_of_origin"); countryOfOrigin != "" {
		product.CountryOfOrigin = countryOfOrigin
	}

	// Dietary flags
	if isGlutenFree := c.PostForm("is_gluten_free"); isGlutenFree != "" {
		product.IsGlutenFree = isGlutenFree == "true"
	}
	if isVegetarian := c.PostForm("is_vegetarian"); isVegetarian != "" {
		product.IsVegetarian = isVegetarian == "true"
	}
	if isVegan := c.PostForm("is_vegan"); isVegan != "" {
		product.IsVegan = isVegan == "true"
	}
	if isAgeRestricted := c.PostForm("is_age_restricted"); isAgeRestricted != "" {
		product.IsAgeRestricted = isAgeRestricted == "true"
	}

	// Additional info
	if allergenInfo := c.PostForm("allergen_info"); allergenInfo != "" {
		product.AllergenInfo = allergenInfo
	}
	if storageType := c.PostForm("storage_type"); storageType != "" {
		product.StorageType = storageType
	}
	if isOwnBrand := c.PostForm("is_own_brand"); isOwnBrand != "" {
		product.IsOwnBrand = isOwnBrand == "true"
	}
	if onlineVisible := c.PostForm("online_visible"); onlineVisible != "" {
		product.OnlineVisible = onlineVisible == "true"
	}
	if status := c.PostForm("status"); status != "" {
		product.Status = status
		fp.IsAvailable = status == "active"
	}
	if notes := c.PostForm("notes"); notes != "" {
		product.Notes = notes
	}
	if packSize := c.PostForm("pack_size"); packSize != "" {
		product.PackSize = packSize
	}

	// Category
	if categoryIDStr := c.PostForm("category_id"); categoryIDStr != "" {
		if categoryUUID, err := uuid.Parse(categoryIDStr); err == nil {
			// Validate category exists
			if err := h.DB.First(&models.Category{}, "id = ?", categoryUUID).Error; err == nil {
				product.CategoryID = categoryUUID
			}
		}
	}

	// Subcategory
	if subcategoryIDStr := c.PostForm("subcategory_id"); subcategoryIDStr != "" {
		if subcategoryUUID, err := uuid.Parse(subcategoryIDStr); err == nil {
			// Validate subcategory exists
			if err := h.DB.First(&models.Subcategory{}, "id = ?", subcategoryUUID).Error; err == nil {
				product.SubcategoryID = &subcategoryUUID
			}
		}
	}

	// Parse multipart form for image handling
	form, err := c.MultipartForm()

	// Start transaction - all database operations should be inside
	tx := h.DB.Begin()

	// Handle image operations if form was parsed successfully
	if err == nil && form != nil {
		files := form.File["images"]
		imagesToDelete := form.Value["delete_images"]

		log.Printf("UpdateProduct: Processing image updates. delete_images: %v, new files count: %d", imagesToDelete, len(files))

		// Step 1: Delete specified images
		// Process all delete_images values - each could be a single UUID or a JSON array string
		for _, imageData := range imagesToDelete {
			imageData = strings.TrimSpace(imageData)
			if imageData == "" {
				continue
			}

			// Collect all image IDs to delete (handle both single ID and JSON array)
			var imageIDsToDelete []string
			if strings.HasPrefix(imageData, "[") {
				// It's a JSON array - parse it
				if err := json.Unmarshal([]byte(imageData), &imageIDsToDelete); err != nil {
					log.Printf("UpdateProduct: Failed to parse delete_images JSON: %v", err)
					continue
				}
			} else {
				// Single ID
				imageIDsToDelete = []string{imageData}
			}

			// Delete each image
			for _, imageID := range imageIDsToDelete {
				imageID = strings.TrimSpace(imageID)
				if imageID == "" {
					continue
				}

				var productImage models.ProductImage
				// Find only non-deleted images that belong to this product
				if err := tx.Where("id = ? AND product_id = ?", imageID, product.ID).First(&productImage).Error; err == nil {
					log.Printf("UpdateProduct: Deleting image ID: %s", imageID)
					// Delete from Firebase
					if h.Storage != nil {
						objectPath, err := utils.ExtractObjectPath(productImage.ImageURL)
						if err == nil && objectPath != "" {
							if err := h.Storage.DeleteFile(objectPath); err != nil {
								log.Println("Failed to delete image from Firebase:", err)
							}
						}
					}
					// Soft delete from database
					if err := tx.Delete(&productImage).Error; err != nil {
						log.Printf("UpdateProduct: Failed to delete image %s: %v", imageID, err)
					}
				} else {
					log.Printf("UpdateProduct: Image not found for ID: %s, error: %v", imageID, err)
				}
			}
		}

		// Also handle removed_image_ids (JSON array format)
		if removedImageIDsJSON := form.Value["removed_image_ids"]; len(removedImageIDsJSON) > 0 {
			for _, removedIDsJSON := range removedImageIDsJSON {
				if removedIDsJSON == "" {
					continue
				}
				var imageIDsToRemove []string
				if err := json.Unmarshal([]byte(removedIDsJSON), &imageIDsToRemove); err != nil {
					log.Printf("UpdateProduct: Failed to parse removed_image_ids JSON: %v", err)
					continue
				}
				log.Printf("UpdateProduct: Parsed removed_image_ids: %v", imageIDsToRemove)
				for _, imageID := range imageIDsToRemove {
					imageID = strings.TrimSpace(imageID)
					if imageID == "" {
						continue
					}
					var productImage models.ProductImage
					if err := tx.Where("id = ? AND product_id = ?", imageID, product.ID).First(&productImage).Error; err == nil {
						log.Printf("UpdateProduct: Deleting image via removed_image_ids: %s", imageID)
						if h.Storage != nil {
							objectPath, err := utils.ExtractObjectPath(productImage.ImageURL)
							if err == nil && objectPath != "" {
								if err := h.Storage.DeleteFile(objectPath); err != nil {
									log.Println("Failed to delete image from Firebase:", err)
								}
							}
						}
						if err := tx.Delete(&productImage).Error; err != nil {
							log.Printf("UpdateProduct: Failed to delete image %s: %v", imageID, err)
						}
					}
				}
			}
		}

		// Step 2: Handle primary image setting (after deletions, before new uploads)
		if primaryImageIDStr := c.PostForm("primary_image_id"); primaryImageIDStr != "" {
			if primaryImageUUID, err := uuid.Parse(primaryImageIDStr); err == nil {
				// Reset all non-deleted images to non-primary
				tx.Model(&models.ProductImage{}).Where("product_id = ? AND deleted_at IS NULL", product.ID).Update("is_primary", false)
				// Set the specified non-deleted image as primary
				tx.Model(&models.ProductImage{}).Where("id = ? AND product_id = ? AND deleted_at IS NULL", primaryImageUUID, product.ID).Update("is_primary", true)
				log.Printf("UpdateProduct: Set primary image to: %s", primaryImageUUID)
			}
		}

		// Step 3: Upload new images if provided
		if len(files) > 0 && h.Storage != nil {
			// Check if there's any primary image among non-deleted images
			var primaryCount int64
			tx.Model(&models.ProductImage{}).Where("product_id = ? AND is_primary = ? AND deleted_at IS NULL", product.ID, true).Count(&primaryCount)

			var newProductImages []models.ProductImage
			for i, fileHeader := range files {
				// Validate file upload
				if err := utils.ValidateFileUpload(fileHeader); err != nil {
					log.Printf("Skipping invalid image: %v", err)
					continue
				}

				file, err := fileHeader.Open()
				if err != nil {
					log.Printf("Failed to open uploaded file: %v", err)
					continue
				}

				imageURL, err := h.Storage.UploadProductImage(
					file,
					fileHeader.Filename,
					fileHeader.Header.Get("Content-Type"),
				)
				file.Close()

				if err != nil {
					log.Printf("Failed to upload image: %v", err)
					continue
				}

				// Make first new image primary if no primary exists
				productImage := models.ProductImage{
					ProductID: product.ID,
					ImageURL:  imageURL,
					IsPrimary: primaryCount == 0 && i == 0,
				}
				newProductImages = append(newProductImages, productImage)
			}

			// Batch create all new images
			if len(newProductImages) > 0 {
				if err := tx.Create(&newProductImages).Error; err != nil {
					log.Printf("Failed to save product images: %v", err)
				} else {
					log.Printf("UpdateProduct: Created %d new images", len(newProductImages))
				}
			}
		}
	}

	// If product was admin-deleted (delisted), restore it by clearing deleted_at and deleted_by
	if wasDelistedByAdmin {
		product.DeletedAt = gorm.DeletedAt{Valid: false}
		product.DeletedBy = ""
		log.Printf("Restoring admin-deleted product %s - franchise user is updating it", product.ID)
	}

	// Clear the Images slice to prevent GORM from trying to save/upsert images
	// Image operations are handled separately above
	product.Images = nil

	// Save product (with Images omitted to prevent GORM from interfering)
	if err := tx.Omit("Images").Save(&product).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update product"})
		return
	}

	// Save franchise product
	if err := tx.Save(&fp).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update franchise product"})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to complete operation"})
		return
	}

	// Reload product with fresh images (only non-deleted)
	h.DB.Unscoped().Preload("Images", "deleted_at IS NULL").Preload("Category").First(&product, product.ID)
	c.JSON(http.StatusOK, product)
}

// DeleteProduct allows franchise owners to delete a product
func (h *FranchiseHandler) DeleteProduct(c *gin.Context) {
	franchiseID, _ := c.Get("franchise_id")
	fID := franchiseID.(uuid.UUID)
	productID := c.Param("id")

	// Find the FranchiseProduct
	var fp models.FranchiseProduct
	if err := h.DB.Where("id = ? AND franchise_id = ?", productID, franchiseID).First(&fp).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found in your franchise"})
		return
	}

	// Get the actual Product with images
	var product models.Product
	if err := h.DB.Unscoped().Preload("Images").First(&product, "id = ?", fp.ProductID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	tx := h.DB.Begin()

	// Delete all product images from Firebase storage
	if h.Storage != nil {
		for _, img := range product.Images {
			objectPath, err := utils.ExtractObjectPath(img.ImageURL)
			if err == nil && objectPath != "" {
				if err := h.Storage.DeleteFile(objectPath); err != nil {
					log.Printf("Failed to delete image %s from Firebase: %v", img.ImageURL, err)
				} else {
					log.Printf("Deleted image from Firebase: %s", img.ImageURL)
				}
			}
			// Delete image record from database
			if err := tx.Delete(&img).Error; err != nil {
				log.Printf("Failed to delete product image record: %v", err)
			}
		}
	}

	// Set deleted_at on franchise_products (soft delete)
	now := time.Now()
	if err := tx.Model(&fp).Update("deleted_at", now).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete franchise product"})
		return
	}

	// Set both deleted_at and deleted_by on the main product
	// This hides the product from ALL portals (admin, franchise, user app)
	product.DeletedAt = gorm.DeletedAt{Time: now, Valid: true}
	product.DeletedBy = fmt.Sprintf("franchise:%s", fID.String())
	if err := tx.Save(&product).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update product deletion info"})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to complete deletion"})
		return
	}

	log.Printf("Franchise %s soft-deleted product %s - images removed from storage", fID, product.ID)

	c.JSON(http.StatusOK, gin.H{
		"message":    "Product deleted successfully",
		"deleted_at": now,
		"deleted_by": product.DeletedBy,
	})
}

// RestoreProduct allows franchise owners to restore an admin-deleted product
// This clears the deleted_at on the products table, making it visible again in all portals
func (h *FranchiseHandler) RestoreProduct(c *gin.Context) {
	franchiseID, _ := c.Get("franchise_id")
	productID := c.Param("id")

	// Find the FranchiseProduct by its ID
	var fp models.FranchiseProduct
	if err := h.DB.Where("id = ? AND franchise_id = ?", productID, franchiseID).First(&fp).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found in your franchise"})
		return
	}

	// Get the actual Product (use Unscoped to include soft-deleted products)
	var product models.Product
	if err := h.DB.Unscoped().First(&product, "id = ?", fp.ProductID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	// Check if product was actually deleted by admin
	if !product.DeletedAt.Valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Product is not deleted"})
		return
	}

	// Restore the product by clearing deleted_at and deleted_by
	product.DeletedAt = gorm.DeletedAt{Valid: false}
	product.DeletedBy = ""

	if err := h.DB.Save(&product).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to restore product"})
		return
	}

	log.Printf("Franchise restored product %s - product is now visible in all portals", product.ID)

	h.DB.Unscoped().Preload("Images", "deleted_at IS NULL").Preload("Category").First(&product, product.ID)
	c.JSON(http.StatusOK, gin.H{
		"message": "Product restored successfully",
		"product": product,
	})
}
