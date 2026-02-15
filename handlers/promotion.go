package handlers

import (
	"log"
	"net/http"
	"time"

	"grabbi-backend/models"
	"grabbi-backend/utils"

	"grabbi-backend/firebase"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PromotionHandler struct {
	DB      *gorm.DB
	Storage firebase.StorageClient
}

func (h *PromotionHandler) GetPromotions(c *gin.Context) {
	var promotions []models.Promotion
	now := time.Now()
	query := h.DB

	// This is a public endpoint. Always filter by active status.
	// Admin users access all promotions (including inactive) via the admin routes.
	query = query.Where("is_active = ?", true)

	// Filter by date range - only show promotions that are currently valid
	query = query.Where("(start_date IS NULL OR start_date <= ?) AND (end_date IS NULL OR end_date >= ?)", now, now)

	if err := query.Find(&promotions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch promotions"})
		return
	}

	c.JSON(http.StatusOK, promotions)
}

// GetAllPromotions returns all promotions (active + inactive) for admin use
func (h *PromotionHandler) GetAllPromotions(c *gin.Context) {
	var promotions []models.Promotion
	if err := h.DB.Order("created_at DESC").Find(&promotions).Error; err != nil {
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

	// Parse start_date and end_date
	if startDateStr := c.PostForm("start_date"); startDateStr != "" {
		if parsedTime, err := time.Parse(time.RFC3339, startDateStr); err == nil {
			promotion.StartDate = &parsedTime
		} else if parsedTime, err := time.Parse("2006-01-02", startDateStr); err == nil {
			promotion.StartDate = &parsedTime
		}
	}
	if endDateStr := c.PostForm("end_date"); endDateStr != "" {
		if parsedTime, err := time.Parse(time.RFC3339, endDateStr); err == nil {
			promotion.EndDate = &parsedTime
		} else if parsedTime, err := time.Parse("2006-01-02", endDateStr); err == nil {
			promotion.EndDate = &parsedTime
		}
	}

	// Image upload
	fileHeader, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Image is required"})
		return
	}

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

	imageURL, err := h.Storage.UploadPromotionImage(
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

	// Parse start_date and end_date
	if startDateStr := c.PostForm("start_date"); startDateStr != "" {
		if parsedTime, err := time.Parse(time.RFC3339, startDateStr); err == nil {
			promotion.StartDate = &parsedTime
		} else if parsedTime, err := time.Parse("2006-01-02", startDateStr); err == nil {
			promotion.StartDate = &parsedTime
		}
	}
	if endDateStr := c.PostForm("end_date"); endDateStr != "" {
		if parsedTime, err := time.Parse(time.RFC3339, endDateStr); err == nil {
			promotion.EndDate = &parsedTime
		} else if parsedTime, err := time.Parse("2006-01-02", endDateStr); err == nil {
			promotion.EndDate = &parsedTime
		}
	}

	//Image update
	fileHeader, err := c.FormFile("image")
	if err == nil {
		// Validate file upload (content type + size)
		if err := utils.ValidateFileUpload(fileHeader); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if promotion.Image != "" {
			objectPath, pathErr := utils.ExtractObjectPath(promotion.Image)
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
			_ = h.Storage.DeleteFile(objectPath)
		}
	}

	if err := h.DB.Delete(&models.Promotion{}, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete promotion"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Promotion deleted successfully"})
}
