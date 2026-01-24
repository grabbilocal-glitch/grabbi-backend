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
	query := h.DB.Preload("Category").Preload("Images")

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

	if err := h.DB.Preload("Category").Preload("Images").Where("id = ?", id).First(&product).Error; err != nil {
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

	product.ID = uuid.New()

	// Handle multiple image uploads
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(400, gin.H{"error": "Failed to parse form"})
		return
	}

	files := form.File["images"]
	if len(files) == 0 {
		c.JSON(400, gin.H{"error": "At least one image is required"})
		return
	}

	// Create product first
	if err := h.DB.Create(&product).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create product"})
		return
	}

	// Upload images and create product image records
	var productImages []models.ProductImage
	for i, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid image"})
			return
		}

		imageURL, err := firebase.UploadProductImage(
			file,
			fileHeader.Filename,
			fileHeader.Header.Get("Content-Type"),
		)
		file.Close()
		
		if err != nil {
			c.JSON(500, gin.H{"error": "Image upload failed"})
			return
		}

		// First image is marked as primary
		productImage := models.ProductImage{
			ProductID: product.ID,
			ImageURL:  imageURL,
			IsPrimary: i == 0,
		}
		productImages = append(productImages, productImage)
	}

	// Save all images
	if err := h.DB.Create(&productImages).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save product images"})
		return
	}

	h.DB.Preload("Category").Preload("Images").First(&product, product.ID)
	c.JSON(http.StatusCreated, product)
}

func (h *ProductHandler) UpdateProduct(c *gin.Context) {
	id := c.Param("id")
	var product models.Product

	if err := h.DB.Preload("Images").Where("id = ?", id).First(&product).Error; err != nil {
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

	// Handle multiple image updates
	form, err := c.MultipartForm()
	if err == nil {
		files := form.File["images"]
		imagesToDelete := form.Value["delete_images"]

		// Delete specified images
		for _, imageID := range imagesToDelete {
			var productImage models.ProductImage
			if err := h.DB.Where("id = ? AND product_id = ?", imageID, product.ID).First(&productImage).Error; err == nil {
				// Delete from Firebase
				objectPath, err := extractObjectPath(productImage.ImageURL)
				if err == nil && objectPath != "" {
					if err := firebase.DeleteFile(objectPath); err != nil {
						log.Println("Failed to delete image from Firebase:", err)
					}
				}
				// Delete from database
				h.DB.Delete(&productImage)
			}
		}

		// Upload new images if provided
		if len(files) > 0 {
			var newProductImages []models.ProductImage
			for i, fileHeader := range files {
				file, err := fileHeader.Open()
				if err != nil {
					c.JSON(400, gin.H{"error": "Invalid image"})
					return
				}

				imageURL, err := firebase.UploadProductImage(
					file,
					fileHeader.Filename,
					fileHeader.Header.Get("Content-Type"),
				)
				file.Close()

				if err != nil {
					c.JSON(500, gin.H{"error": "Image upload failed"})
					return
				}

				productImage := models.ProductImage{
					ProductID: product.ID,
					ImageURL:  imageURL,
					IsPrimary: len(product.Images) == 0 && i == 0,
				}
				newProductImages = append(newProductImages, productImage)
			}

			if err := h.DB.Create(&newProductImages).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save product images"})
				return
			}
		}
	}

	if primaryImageID := c.PostForm("primary_image_id"); primaryImageID != "" {
		// Reset all primary flags for this product
		h.DB.Model(&models.ProductImage{}).
			Where("product_id = ?", product.ID).
			Update("is_primary", false)
		
		// Set selected image as primary
		h.DB.Model(&models.ProductImage{}).
			Where("id = ? AND product_id = ?", primaryImageID, product.ID).
			Update("is_primary", true)
	}

	if err := h.DB.Save(&product).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update product"})
		return
	}

	h.DB.Preload("Category").Preload("Images").First(&product, product.ID)
	c.JSON(http.StatusOK, product)
}

func (h *ProductHandler) DeleteProduct(c *gin.Context) {
	id := c.Param("id")
	var product models.Product

	if err := h.DB.Preload("Images").First(&product, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	// Check and delete product images
	for _, productImage := range product.Images {
		// Check if this image is referenced in any order
		var orderImageCount int64
		h.DB.Model(&models.OrderItem{}).
			Where("image_url = ?", productImage.ImageURL).
			Count(&orderImageCount)

		if orderImageCount > 0 {
			// Image is used in orders - keep in Firebase
			log.Printf("Image %s is referenced in %d order(s) - preserving in Firebase storage", 
				productImage.ImageURL, orderImageCount)
		} else {
			// Image not used in any order - safe to delete from Firebase
			objectPath, err := extractObjectPath(productImage.ImageURL)
			if err == nil && objectPath != "" {
				if err := firebase.DeleteFile(objectPath); err != nil {
					log.Printf("Failed to delete image %s from Firebase: %v", productImage.ImageURL, err)
				} else {
					log.Printf("Deleted image from Firebase: %s", productImage.ImageURL)
				}
			}
		}

		// Always delete the product_image record from database
		if err := h.DB.Delete(&productImage).Error; err != nil {
			log.Printf("Failed to delete product image record %s: %v", productImage.ID, err)
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

	query := h.DB.Preload("Category").Preload("Images")

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
