package handlers

import (
	"net/http"

	"grabbi-backend/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SubcategoryHandler struct {
	DB *gorm.DB
}

// GetSubcategories retrieves all subcategories
func (h *SubcategoryHandler) GetSubcategories(c *gin.Context) {
	var subcategories []models.Subcategory
	if err := h.DB.Preload("Category").Find(&subcategories).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch subcategories"})
		return
	}

	c.JSON(http.StatusOK, subcategories)
}

// CreateSubcategory creates a new subcategory
func (h *SubcategoryHandler) CreateSubcategory(c *gin.Context) {
	var subcategory models.Subcategory
	if err := c.ShouldBindJSON(&subcategory); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate that the parent category exists
	var category models.Category
	if err := h.DB.Where("id = ?", subcategory.CategoryID).First(&category).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Parent category not found"})
		return
	}

	subcategory.ID = uuid.New()
	if err := h.DB.Create(&subcategory).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create subcategory"})
		return
	}

	// Return the subcategory with preloaded category
	if err := h.DB.Preload("Category").First(&subcategory, "id = ?", subcategory.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch created subcategory"})
		return
	}

	c.JSON(http.StatusCreated, subcategory)
}

// UpdateSubcategory updates an existing subcategory
func (h *SubcategoryHandler) UpdateSubcategory(c *gin.Context) {
	id := c.Param("id")
	var subcategory models.Subcategory

	if err := h.DB.Where("id = ?", id).First(&subcategory).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Subcategory not found"})
		return
	}

	if err := c.ShouldBindJSON(&subcategory); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate that the parent category exists if it's being changed
	if err := h.DB.Where("id = ?", subcategory.CategoryID).First(&models.Category{}).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Parent category not found"})
		return
	}

	if err := h.DB.Save(&subcategory).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update subcategory"})
		return
	}

	c.JSON(http.StatusOK, subcategory)
}

// DeleteSubcategory deletes a subcategory
func (h *SubcategoryHandler) DeleteSubcategory(c *gin.Context) {
	id := c.Param("id")
	
	// Check if subcategory has associated products
	var productCount int64
	if err := h.DB.Model(&models.Product{}).Where("subcategory_id = ?", id).Count(&productCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check subcategory dependencies"})
		return
	}
	
	if productCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Cannot delete subcategory with associated products",
			"message": "Please reassign or delete associated products first",
			"product_count": productCount,
		})
		return
	}
	
	// Safe to delete
	if err := h.DB.Delete(&models.Subcategory{}, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete subcategory"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Subcategory deleted successfully"})
}
