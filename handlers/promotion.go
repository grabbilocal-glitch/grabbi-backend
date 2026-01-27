package handlers

import (
	"net/http"

	"grabbi-backend/models"
	"grabbi-backend/utils"

	"grabbi-backend/firebase"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PromotionHandler struct {
	DB *gorm.DB
}

func (h *PromotionHandler) GetPromotions(c *gin.Context) {
	var promotions []models.Promotion
	query := h.DB

	// Only show active promotions for non-admin users
	userRole, exists := c.Get("user_role")
	if !exists || userRole != "admin" {
		query = query.Where("is_active = ?", true)
	}

	if err := query.Find(&promotions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch promotions"})
		return
	}

	c.JSON(http.StatusOK, promotions)
}

func (h *PromotionHandler) GetPromotion(c *gin.Context) {
	id := c.Param("id")
	var promotion models.Promotion

	if err := h.DB.Where("id = ?", id).First(&promotion).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Promotion not found"})
		return
	}

	c.JSON(http.StatusOK, promotion)
}

func (h *PromotionHandler) CreatePromotion(c *gin.Context) {
	var promotion models.Promotion

	promotion.ID = uuid.New()
	promotion.Title = c.PostForm("title")
	promotion.Description = c.PostForm("description")
	promotion.ProductURL = c.PostForm("product_url")
	promotion.IsActive = c.PostForm("is_active") == "true"

	// Image upload
	fileHeader, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Image is required"})
		return
	}

	file, _ := fileHeader.Open()
	defer file.Close()

	imageURL, err := firebase.UploadPromotionImage(
		file,
		fileHeader.Filename,
		fileHeader.Header.Get("Content-Type"),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Image upload failed"})
		return
	}

	promotion.Image = imageURL

	if err := h.DB.Create(&promotion).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create promotion"})
		return
	}

	c.JSON(http.StatusCreated, promotion)
}

func (h *PromotionHandler) UpdatePromotion(c *gin.Context) {
	id := c.Param("id")
	var promotion models.Promotion

	if err := h.DB.Where("id = ?", id).First(&promotion).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Promotion not found"})
		return
	}

	promotion.Title = c.PostForm("title")
	promotion.Description = c.PostForm("description")
	promotion.ProductURL = c.PostForm("product_url")
	promotion.IsActive = c.PostForm("is_active") == "true"

	//Image update
	fileHeader, err := c.FormFile("image")
	if err == nil {
		if promotion.Image != "" {
			objectPath, err := utils.ExtractObjectPath(promotion.Image)
			if err == nil {
				_ = firebase.DeleteFile(objectPath)
			}
		}

		// 2️⃣ Upload new image
		file, _ := fileHeader.Open()
		defer file.Close()

		imageURL, err := firebase.UploadPromotionImage(
			file,
			fileHeader.Filename,
			fileHeader.Header.Get("Content-Type"),
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Image upload failed"})
			return
		}

		promotion.Image = imageURL
	}

	if err := h.DB.Save(&promotion).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update promotion"})
		return
	}

	c.JSON(http.StatusOK, promotion)
}

func (h *PromotionHandler) DeletePromotion(c *gin.Context) {
	id := c.Param("id")
	var promotion models.Promotion

	if err := h.DB.Where("id = ?", id).First(&promotion).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Promotion not found"})
		return
	}

	// Delete image from Firebase
	if promotion.Image != "" {
		objectPath, err := utils.ExtractObjectPath(promotion.Image)
		if err == nil {
			_ = firebase.DeleteFile(objectPath)
		}
	}

	if err := h.DB.Delete(&models.Promotion{}, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete promotion"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Promotion deleted successfully"})
}
