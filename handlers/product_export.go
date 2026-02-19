package handlers

import (
	"net/http"
	"strings"

	"grabbi-backend/models"

	"github.com/gin-gonic/gin"
)

// ProductExportResponse represents a product with franchise names for Excel export
type ProductExportResponse struct {
	models.Product
	FranchiseNames string `json:"franchise_names"`
}

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

	// Build response with franchise names for each product
	var response []ProductExportResponse
	for _, product := range products {
		// Fetch franchise names for this product through FranchiseProduct
		var franchiseNames []string
		h.DB.Table("franchises").
			Select("franchises.name").
			Joins("JOIN franchise_products ON franchises.id = franchise_products.franchise_id").
			Where("franchise_products.product_id = ? AND franchise_products.deleted_at IS NULL", product.ID).
			Pluck("franchises.name", &franchiseNames)

		// Join franchise names with newline for Excel import/export compatibility
		franchiseNamesStr := strings.Join(franchiseNames, "\n")

		response = append(response, ProductExportResponse{
			Product:        product,
			FranchiseNames: franchiseNamesStr,
		})
	}

	c.JSON(http.StatusOK, response)
}
