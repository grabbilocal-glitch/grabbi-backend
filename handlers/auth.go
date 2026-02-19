package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"

	"grabbi-backend/models"
	"grabbi-backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthHandler struct {
	DB *gorm.DB
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=8"`
		Name     string `json:"name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": utils.SanitizeValidationError(err)})
		return
	}

	var existingUser models.User
	if err := h.DB.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already registered"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	user := models.User{
		ID:       uuid.New(),
		Email:    req.Email,
		Password: string(hashedPassword),
		Name:     req.Name,
		Role:     "customer",
	}

	if err := h.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	token, err := utils.GenerateToken(user.ID, user.Email, user.Role, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	refreshToken, err := utils.GenerateRefreshToken(user.ID, user.Email, user.Role, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate refresh token"})
		return
	}

	// Store refresh token
	rt := models.RefreshToken{
		UserID:    user.ID,
		Token:     refreshToken,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}
	h.DB.Create(&rt)

	// Send welcome email (non-blocking)
	utils.SendWelcomeEmail(user.Email, user.Name)

	c.JSON(http.StatusCreated, gin.H{
		"token":         token,
		"refresh_token": refreshToken,
		"user": gin.H{
			"id":             user.ID,
			"email":          user.Email,
			"name":           user.Name,
			"role":           user.Role,
			"loyalty_points": user.LoyaltyPoints,
		},
	})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": utils.SanitizeValidationError(err)})
		return
	}

	var user models.User
	if err := h.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Check if user is blocked
	if user.IsBlocked {
		c.JSON(http.StatusForbidden, gin.H{"error": "Your account has been blocked. Please contact support."})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	token, err := utils.GenerateToken(user.ID, user.Email, user.Role, user.FranchiseID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	refreshToken, err := utils.GenerateRefreshToken(user.ID, user.Email, user.Role, user.FranchiseID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate refresh token"})
		return
	}

	// Store refresh token
	rt := models.RefreshToken{
		UserID:    user.ID,
		Token:     refreshToken,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}
	h.DB.Create(&rt)

	response := gin.H{
		"token":         token,
		"refresh_token": refreshToken,
		"user": gin.H{
			"id":             user.ID,
			"email":          user.Email,
			"name":           user.Name,
			"role":           user.Role,
			"loyalty_points": user.LoyaltyPoints,
			"franchise_id":   user.FranchiseID,
			"phone":          user.Phone,
		},
	}

	if user.FranchiseID != nil {
		var franchise models.Franchise
		if err := h.DB.Where("id = ?", user.FranchiseID).First(&franchise).Error; err == nil {
			response["franchise"] = gin.H{
				"id":   franchise.ID,
				"name": franchise.Name,
				"slug": franchise.Slug,
			}
		}
	}

	c.JSON(http.StatusOK, response)
}

func (h *AuthHandler) GetProfile(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var user models.User
	if err := h.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	response := gin.H{
		"id":             user.ID,
		"email":          user.Email,
		"name":           user.Name,
		"role":           user.Role,
		"loyalty_points": user.LoyaltyPoints,
		"franchise_id":   user.FranchiseID,
		"phone":          user.Phone,
	}

	if user.FranchiseID != nil {
		var franchise models.Franchise
		if err := h.DB.Preload("StoreHours").Where("id = ?", user.FranchiseID).First(&franchise).Error; err == nil {
			response["franchise"] = franchise
		}
	}

	c.JSON(http.StatusOK, response)
}

func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": utils.SanitizeValidationError(err)})
		return
	}

	// Always return 200 to prevent email enumeration
	successMsg := gin.H{"message": "If an account with that email exists, a password reset link has been sent."}

	var user models.User
	if err := h.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusOK, successMsg)
		return
	}

	// Generate reset token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate reset token"})
		return
	}
	token := hex.EncodeToString(tokenBytes)

	resetToken := models.PasswordResetToken{
		UserID:    user.ID,
		Token:     token,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}

	if err := h.DB.Create(&resetToken).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create reset token"})
		return
	}

	// Select the appropriate frontend URL based on user role
	var frontendURL string
	switch user.Role {
	case "admin":
		frontendURL = os.Getenv("ADMIN_URL")
		if frontendURL == "" {
			frontendURL = "http://localhost:5174"
		}
	case "franchise_owner", "franchise_staff":
		frontendURL = os.Getenv("FRANCHISE_URL")
		if frontendURL == "" {
			frontendURL = "http://localhost:5175"
		}
	default:
		frontendURL = os.Getenv("FRONTEND_URL")
		if frontendURL == "" {
			frontendURL = "http://localhost:5173"
		}
	}
	utils.SendPasswordResetEmail(user.Email, user.Name, token, frontendURL)

	c.JSON(http.StatusOK, successMsg)
}

func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req struct {
		Token    string `json:"token" binding:"required"`
		Password string `json:"password" binding:"required,min=8"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": utils.SanitizeValidationError(err)})
		return
	}

	var resetToken models.PasswordResetToken
	if err := h.DB.Where("token = ? AND used_at IS NULL AND expires_at > ?", req.Token, time.Now()).First(&resetToken).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired reset token"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	now := time.Now()
	h.DB.Model(&resetToken).Update("used_at", &now)
	h.DB.Model(&models.User{}).Where("id = ?", resetToken.UserID).Update("password", string(hashedPassword))

	c.JSON(http.StatusOK, gin.H{"message": "Password reset successfully"})
}

func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req struct {
		Name  *string `json:"name"`
		Phone *string `json:"phone"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": utils.SanitizeValidationError(err)})
		return
	}

	var user models.User
	if err := h.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if req.Name != nil {
		user.Name = *req.Name
	}
	if req.Phone != nil {
		user.Phone = *req.Phone
	}

	if err := h.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":             user.ID,
		"email":          user.Email,
		"name":           user.Name,
		"role":           user.Role,
		"phone":          user.Phone,
		"loyalty_points": user.LoyaltyPoints,
	})
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=8"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": utils.SanitizeValidationError(err)})
		return
	}

	var user models.User
	if err := h.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Current password is incorrect"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	h.DB.Model(&user).Update("password", string(hashedPassword))
	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
}

func (h *AuthHandler) ListUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	query := h.DB.Model(&models.User{})

	if role := c.Query("role"); role != "" {
		query = query.Where("role = ?", role)
	}
	if search := c.Query("search"); search != "" {
		query = query.Where("LOWER(name) LIKE LOWER(?) OR LOWER(email) LIKE LOWER(?)", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	query.Count(&total)

	var users []models.User
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
		return
	}

	type UserResponse struct {
		ID            uuid.UUID  `json:"id"`
		Email         string     `json:"email"`
		Name          string     `json:"name"`
		Role          string     `json:"role"`
		Phone         string     `json:"phone"`
		IsBlocked     bool       `json:"is_blocked"`
		LoyaltyPoints int        `json:"loyalty_points"`
		FranchiseID   *uuid.UUID `json:"franchise_id,omitempty"`
		CreatedAt     time.Time  `json:"created_at"`
	}

	var result []UserResponse
	for _, u := range users {
		result = append(result, UserResponse{
			ID:            u.ID,
			Email:         u.Email,
			Name:          u.Name,
			Role:          u.Role,
			Phone:         u.Phone,
			IsBlocked:     u.IsBlocked,
			LoyaltyPoints: u.LoyaltyPoints,
			FranchiseID:   u.FranchiseID,
			CreatedAt:     u.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"users": result,
		"total": total,
		"page":  page,
		"limit": limit,
		"pages": int(math.Ceil(float64(total) / float64(limit))),
	})
}

func (h *AuthHandler) GetUser(c *gin.Context) {
	id := c.Param("id")

	var user models.User
	if err := h.DB.Where("id = ?", id).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":             user.ID,
		"email":          user.Email,
		"name":           user.Name,
		"role":           user.Role,
		"phone":          user.Phone,
		"is_blocked":     user.IsBlocked,
		"loyalty_points": user.LoyaltyPoints,
		"franchise_id":   user.FranchiseID,
		"created_at":     user.CreatedAt,
	})
}

func (h *AuthHandler) UpdateUser(c *gin.Context) {
	id := c.Param("id")
	currentUserID, _ := c.Get("user_id")

	userUUID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var user models.User
	if err := h.DB.Where("id = ?", id).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var req struct {
		Role      *string `json:"role"`
		IsBlocked *bool   `json:"is_blocked"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": utils.SanitizeValidationError(err)})
		return
	}

	// Validate role if provided
	validRoles := map[string]bool{"customer": true, "franchise_owner": true, "franchise_staff": true, "admin": true}
	if req.Role != nil {
		if !validRoles[*req.Role] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role"})
			return
		}
		if currentUserID.(uuid.UUID) == userUUID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot change your own role"})
			return
		}
	}

	// Use targeted Updates instead of Save to avoid constraint issues on unrelated fields
	updates := map[string]interface{}{}
	if req.Role != nil {
		updates["role"] = *req.Role
	}
	if req.IsBlocked != nil {
		updates["is_blocked"] = *req.IsBlocked
	}

	if len(updates) > 0 {
		if err := h.DB.Model(&user).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
			return
		}
	}

	// Re-read user to get updated values
	h.DB.Where("id = ?", id).First(&user)

	c.JSON(http.StatusOK, gin.H{
		"id":             user.ID,
		"email":          user.Email,
		"name":           user.Name,
		"role":           user.Role,
		"phone":          user.Phone,
		"is_blocked":     user.IsBlocked,
		"loyalty_points": user.LoyaltyPoints,
		"franchise_id":   user.FranchiseID,
		"created_at":     user.CreatedAt,
	})
}

func (h *AuthHandler) RedeemPoints(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req struct {
		Points int `json:"points" binding:"required,min=1"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": utils.SanitizeValidationError(err)})
		return
	}

	var user models.User
	if err := h.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if user.LoyaltyPoints < req.Points {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Insufficient points. You have %d points.", user.LoyaltyPoints)})
		return
	}

	user.LoyaltyPoints -= req.Points
	h.DB.Save(&user)

	history := models.LoyaltyHistory{
		UserID:      user.ID,
		Points:      req.Points,
		Type:        "redeemed",
		Description: fmt.Sprintf("Redeemed %d points", req.Points),
	}
	h.DB.Create(&history)

	c.JSON(http.StatusOK, gin.H{
		"message":          "Points redeemed successfully",
		"points_redeemed":  req.Points,
		"remaining_points": user.LoyaltyPoints,
	})
}

func (h *AuthHandler) GetLoyaltyHistory(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	var total int64
	h.DB.Model(&models.LoyaltyHistory{}).Where("user_id = ?", userID).Count(&total)

	var history []models.LoyaltyHistory
	if err := h.DB.Where("user_id = ?", userID).Order("created_at DESC").Offset(offset).Limit(limit).Find(&history).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch loyalty history"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"history": history,
		"total":   total,
		"page":    page,
		"limit":   limit,
	})
}

func (h *AuthHandler) RefreshTokenHandler(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": utils.SanitizeValidationError(err)})
		return
	}

	// Find the refresh token
	var rt models.RefreshToken
	if err := h.DB.Where("token = ? AND revoked_at IS NULL AND expires_at > ?", req.RefreshToken, time.Now()).First(&rt).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired refresh token"})
		return
	}

	// Revoke old refresh token
	now := time.Now()
	h.DB.Model(&rt).Update("revoked_at", &now)

	// Get user
	var user models.User
	if err := h.DB.Where("id = ?", rt.UserID).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	if user.IsBlocked {
		c.JSON(http.StatusForbidden, gin.H{"error": "Your account has been blocked"})
		return
	}

	// Generate new tokens
	newToken, err := utils.GenerateToken(user.ID, user.Email, user.Role, user.FranchiseID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	newRefreshToken, err := utils.GenerateRefreshToken(user.ID, user.Email, user.Role, user.FranchiseID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate refresh token"})
		return
	}

	// Store new refresh token
	newRT := models.RefreshToken{
		UserID:    user.ID,
		Token:     newRefreshToken,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}
	h.DB.Create(&newRT)

	c.JSON(http.StatusOK, gin.H{
		"token":         newToken,
		"refresh_token": newRefreshToken,
	})
}
