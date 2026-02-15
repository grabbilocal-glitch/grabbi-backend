package handlers

import (
	"net/http"

	"grabbi-backend/models"

	"github.com/gin-gonic/gin"
)

// GetProductsExport returns ALL products for Excel export
func (h *ProductHandler) GetProductsExport(c *gin.Context) {
	var products []models.Product

	// Fetch all active products with categories and images - no limit
	if err := h.DB.Preload("Category").Preload("Subcategory").Preload("Images").
		Where("deleted_at IS NULL").
		Find(&products).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch products"})
		return
	}

	c.JSON(http.StatusOK, products)
}