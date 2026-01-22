package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"grabbi-backend/firebase"
	"grabbi-backend/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ProductHandler struct {
	DB *gorm.DB
}

func (h *ProductHandler) GetProducts(c *gin.Context) {
	var products []models.Product
	query := h.DB.Preload("Category")

	// Filter by category
	if categoryID := c.Query("category_id"); categoryID != "" {
		query = query.Where("category_id = ?", categoryID)
	}

	// Search by name
	if search := c.Query("search"); search != "" {
		query = query.Where("name ILIKE ?", "%"+search+"%")
	}

	if err := query.Find(&products).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch products"})
		return
	}

	c.JSON(http.StatusOK, products)
}

func (h *ProductHandler) GetProduct(c *gin.Context) {
	id := c.Param("id")
	var product models.Product

	if err := h.DB.Preload("Category").Where("id = ?", id).First(&product).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}
	c.JSON(http.StatusOK, product)
}

func (h *ProductHandler) CreateProduct(c *gin.Context) {
	var product models.Product

	// Form values
	product.Name = c.PostForm("name")
	product.Description = c.PostForm("description")
	product.Brand = c.PostForm("brand")
	product.PackSize = c.PostForm("pack_size")

	product.Price, _ = strconv.ParseFloat(c.PostForm("price"), 64)
	product.Stock, _ = strconv.Atoi(c.PostForm("stock"))
	product.IsVegan = c.PostForm("is_vegan") == "true"
	product.IsGlutenFree = c.PostForm("is_gluten_free") == "true"

	categoryID, err := uuid.Parse(c.PostForm("category_id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid category ID"})
		return
	}
	product.CategoryID = categoryID

	// Validate category
	if err := h.DB.First(&models.Category{}, "id = ?", product.CategoryID).Error; err != nil {
		c.JSON(400, gin.H{"error": "Invalid category"})
		return
	}

	// Image upload
	fileHeader, err := c.FormFile("image")
	if err != nil {
		c.JSON(400, gin.H{"error": "Image is required"})
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid image"})
		return
	}
	defer file.Close()

	imageURL, err := firebase.UploadProductImage(
		file,
		fileHeader.Filename,
		fileHeader.Header.Get("Content-Type"),
	)
	if err != nil {
		c.JSON(500, gin.H{"error": "Image upload failed"})
		return
	}

	product.Image = imageURL
	product.ID = uuid.New()

	if err := h.DB.Create(&product).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create product"})
		return
	}

	h.DB.Preload("Category").First(&product, product.ID)
	c.JSON(http.StatusCreated, product)
}

func (h *ProductHandler) UpdateProduct(c *gin.Context) {
	id := c.Param("id")
	var product models.Product

	if err := h.DB.Where("id = ?", id).First(&product).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	product.Name = c.PostForm("name")
	product.Description = c.PostForm("description")
	product.Brand = c.PostForm("brand")
	product.PackSize = c.PostForm("pack_size")

	if price := c.PostForm("price"); price != "" {
		product.Price, _ = strconv.ParseFloat(price, 64)
	}

	if stock := c.PostForm("stock"); stock != "" {
		product.Stock, _ = strconv.Atoi(stock)
	}

	product.IsVegan = c.PostForm("is_vegan") == "true"
	product.IsGlutenFree = c.PostForm("is_gluten_free") == "true"

	// Image update
	fileHeader, err := c.FormFile("image")
	if err == nil {
		// Delete old image from Firebase if exists
		if product.Image != "" {
			objectPath, err := extractObjectPath(product.Image)
			if err == nil && objectPath != "" {
				if err := firebase.DeleteFile(objectPath); err != nil {
					log.Println("Failed to delete old image from Firebase:", err)
				} else {
					log.Println("Old image deleted from Firebase:", objectPath)
				}
			}
		}

		// Upload new image
		file, _ := fileHeader.Open()
		defer file.Close()

		imageURL, err := firebase.UploadProductImage(
			file,
			fileHeader.Filename,
			fileHeader.Header.Get("Content-Type"),
		)
		if err != nil {
			c.JSON(500, gin.H{"error": "Image upload failed"})
			return
		}

		product.Image = imageURL
	}

	if err := h.DB.Save(&product).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update product"})
		return
	}

	h.DB.Preload("Category").First(&product, product.ID)
	c.JSON(http.StatusOK, product)
}

func (h *ProductHandler) DeleteProduct(c *gin.Context) {
	id := c.Param("id")
	var product models.Product

	if err := h.DB.First(&product, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	// Delete image from Firebase
	if product.Image != "" {
		objectPath, err := extractObjectPath(product.Image)
		if err == nil && objectPath != "" {
			if err := firebase.DeleteFile(objectPath); err != nil {
				log.Println("Failed to delete image from Firebase:", err)
			}
		}
	}

	if err := h.DB.Delete(&models.Product{}, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete product"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Product deleted successfully"})
}

func (h *ProductHandler) GetProductsPaginated(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset := (page - 1) * limit

	var products []models.Product
	var total int64

	query := h.DB.Preload("Category")

	// Filter by category
	if categoryID := c.Query("category_id"); categoryID != "" {
		query = query.Where("category_id = ?", categoryID)
	}

	// Search by name
	if search := c.Query("search"); search != "" {
		query = query.Where("name ILIKE ?", "%"+search+"%")
	}

	query.Model(&models.Product{}).Count(&total)
	query.Offset(offset).Limit(limit).Find(&products)

	c.JSON(http.StatusOK, gin.H{
		"products": products,
		"total":    total,
		"page":     page,
		"limit":    limit,
	})
}

// extractObjectPath extracts the storage object path from the full URL
func extractObjectPath(url string) (string, error) {
	const prefix = "https://storage.googleapis.com/"
	if !strings.HasPrefix(url, prefix) {
		return "", fmt.Errorf("invalid URL")
	}

	// Remove the prefix and bucket name
	path := strings.TrimPrefix(url, prefix)
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid URL format")
	}

	return parts[1], nil
}
