package handlers

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"grabbi-backend/firebase"
	"grabbi-backend/models"
	"grabbi-backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ProductHandler struct {
	DB      *gorm.DB
	Storage firebase.StorageClient
}

// generateSKU generates a unique SKU using database sequence, with Go-level fallback
func generateSKU(db *gorm.DB) (string, error) {
	var sku string
	if err := db.Raw("SELECT generate_next_sku()").Scan(&sku).Error; err != nil {
		// Fallback: generate SKU using timestamp + random suffix
		log.Printf("DB SKU generation failed, using fallback: %v", err)
		sku = fmt.Sprintf("GRB-%d%04d", time.Now().Unix()%100000, rand.Intn(10000))
		return sku, nil
	}
	return sku, nil
}

func (h *ProductHandler) GetProducts(c *gin.Context) {
	// If franchise_id provided, return products with franchise overrides merged
	if franchiseID := c.Query("franchise_id"); franchiseID != "" {
		var fps []models.FranchiseProduct
		fpQuery := h.DB.Preload("Product").Preload("Product.Category").Preload("Product.Images").
			Where("franchise_id = ? AND is_available = ?", franchiseID, true)

		if categoryID := c.Query("category_id"); categoryID != "" {
			fpQuery = fpQuery.Joins("JOIN products ON products.id = franchise_products.product_id").
				Where("products.category_id = ?", categoryID)
		}
		if search := c.Query("search"); search != "" {
			fpQuery = fpQuery.Joins("JOIN products p2 ON p2.id = franchise_products.product_id").
				Where("LOWER(p2.item_name) LIKE LOWER(?)", "%"+search+"%")
		}

		if err := fpQuery.Find(&fps).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch products"})
			return
		}

		// Return products with overrides applied
		var products []models.Product
		for _, fp := range fps {
			p := fp.Product
			if fp.RetailPriceOverride != nil {
				p.RetailPrice = *fp.RetailPriceOverride
			}
			if fp.PromotionPriceOverride != nil {
				p.PromotionPrice = fp.PromotionPriceOverride
			}
			p.StockQuantity = fp.StockQuantity
			p.ReorderLevel = fp.ReorderLevel
			if fp.ShelfLocation != "" {
				p.ShelfLocation = fp.ShelfLocation
			}
			products = append(products, p)
		}
		c.JSON(http.StatusOK, products)
		return
	}

	// Default: return master catalog
	var products []models.Product
	query := h.DB.Preload("Category").Preload("Images")

	// Filter by category
	if categoryID := c.Query("category_id"); categoryID != "" {
		query = query.Where("category_id = ?", categoryID)
	}

	// Filter by online_visible status
	if c.Query("show_all") != "true" {
		query = query.Where("online_visible = ?", true)
	}

	// Filter by active status
	query = query.Where("status = ?", "active")

	// Search by name
	if search := c.Query("search"); search != "" {
		query = query.Where("LOWER(item_name) LIKE LOWER(?)", "%"+search+"%")
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

	if err := h.DB.Preload("Category").Preload("Images").Where("id = ? AND status = ?", id, "active").First(&product).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}
	c.JSON(http.StatusOK, product)
}

func (h *ProductHandler) CreateProduct(c *gin.Context) {
	var product models.Product

	// Basic Info
	product.SKU = c.PostForm("sku")

	// Auto-generate SKU if empty
	if product.SKU == "" {
		generatedSKU, err := generateSKU(h.DB)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate SKU"})
			return
		}
		product.SKU = generatedSKU
		log.Printf("Auto-generated SKU: %s", generatedSKU)
	}

	product.ItemName = c.PostForm("item_name")
	product.ShortDescription = c.PostForm("short_description")
	product.LongDescription = c.PostForm("long_description")
	product.Brand = c.PostForm("brand")
	product.PackSize = c.PostForm("pack_size")

	// Pricing
	product.CostPrice, _ = strconv.ParseFloat(c.PostForm("cost_price"), 64)
	product.RetailPrice, _ = strconv.ParseFloat(c.PostForm("retail_price"), 64)

	if promoPrice := c.PostForm("promotion_price"); promoPrice != "" {
		price, _ := strconv.ParseFloat(promoPrice, 64)
		product.PromotionPrice = &price
	}

	// Parse date fields (format: YYYY-MM-DD)
	if promoStart := c.PostForm("promotion_start"); promoStart != "" {
		if parsedTime, err := time.Parse("2006-01-02", promoStart); err == nil {
			product.PromotionStart = &parsedTime
		} else {
			log.Printf("Failed to parse promotion_start: %s, error: %v", promoStart, err)
		}
	}

	if promoEnd := c.PostForm("promotion_end"); promoEnd != "" {
		if parsedTime, err := time.Parse("2006-01-02", promoEnd); err == nil {
			product.PromotionEnd = &parsedTime
		} else {
			log.Printf("Failed to parse promotion_end: %s, error: %v", promoEnd, err)
		}
	}

	if expiryDate := c.PostForm("expiry_date"); expiryDate != "" {
		if parsedTime, err := time.Parse("2006-01-02", expiryDate); err == nil {
			product.ExpiryDate = &parsedTime
		} else {
			log.Printf("Failed to parse expiry_date: %s, error: %v", expiryDate, err)
		}
	}

	product.GrossMargin, _ = strconv.ParseFloat(c.PostForm("gross_margin"), 64)
	product.StaffDiscount, _ = strconv.ParseFloat(c.PostForm("staff_discount"), 64)
	product.TaxRate, _ = strconv.ParseFloat(c.PostForm("tax_rate"), 64)

	// Inventory
	product.StockQuantity, _ = strconv.Atoi(c.PostForm("stock_quantity"))
	product.ReorderLevel, _ = strconv.Atoi(c.PostForm("reorder_level"))
	product.ShelfLocation = c.PostForm("shelf_location")

	product.WeightVolume, _ = strconv.ParseFloat(c.PostForm("weight_volume"), 64)
	product.UnitOfMeasure = c.PostForm("unit_of_measure")

	// Dietary flags
	product.IsVegan = c.PostForm("is_vegan") == "true"
	product.IsGlutenFree = c.PostForm("is_gluten_free") == "true"
	product.IsVegetarian = c.PostForm("is_vegetarian") == "true"
	product.IsAgeRestricted = c.PostForm("is_age_restricted") == "true"

	if minAge := c.PostForm("minimum_age"); minAge != "" {
		age, _ := strconv.Atoi(minAge)
		product.MinimumAge = &age
	}

	// Additional Info
	product.Supplier = c.PostForm("supplier")
	product.CountryOfOrigin = c.PostForm("country_of_origin")
	product.AllergenInfo = c.PostForm("allergen_info")
	product.StorageType = c.PostForm("storage_type")
	product.IsOwnBrand = c.PostForm("is_own_brand") == "true"
	product.OnlineVisible = c.PostForm("online_visible") == "true"
	product.Status = c.PostForm("status")
	product.Notes = c.PostForm("notes")
	product.Barcode = c.PostForm("barcode")
	product.BatchNumber = c.PostForm("batch_number")

	// Category
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

	// Subcategory (optional)
	if subcategoryIDStr := c.PostForm("subcategory_id"); subcategoryIDStr != "" {
		subcategoryID, err := uuid.Parse(subcategoryIDStr)
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid subcategory ID"})
			return
		}
		product.SubcategoryID = &subcategoryID
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
		// Validate file upload (content type + size)
		if err := utils.ValidateFileUpload(fileHeader); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		file, err := fileHeader.Open()
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid image"})
			return
		}

		imageURL, err := h.Storage.UploadProductImage(
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

	// Handle franchise associations
	if franchiseIDsStr := c.PostForm("franchise_ids"); franchiseIDsStr != "" {
		franchiseIDs := strings.Split(franchiseIDsStr, ",")
		for _, fidStr := range franchiseIDs {
			fidStr = strings.TrimSpace(fidStr)
			if fidStr == "" {
				continue
			}
			parsedFID, err := uuid.Parse(fidStr)
			if err != nil {
				log.Printf("Invalid franchise ID in create product: %s", fidStr)
				continue
			}
			fp := models.FranchiseProduct{
				FranchiseID:   parsedFID,
				ProductID:     product.ID,
				StockQuantity: product.StockQuantity,
				ReorderLevel:  product.ReorderLevel,
				IsAvailable:   product.Status == "active",
			}
			if err := h.DB.Create(&fp).Error; err != nil {
				log.Printf("Failed to create franchise product link for franchise %s: %v", fidStr, err)
			}
		}
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

	// Basic Info
	if sku := c.PostForm("sku"); sku != "" {
		product.SKU = sku
	}
	product.ItemName = c.PostForm("item_name")
	product.ShortDescription = c.PostForm("short_description")
	product.LongDescription = c.PostForm("long_description")
	product.Brand = c.PostForm("brand")
	product.PackSize = c.PostForm("pack_size")

	// Pricing
	if price := c.PostForm("retail_price"); price != "" {
		product.RetailPrice, _ = strconv.ParseFloat(price, 64)
	}
	if costPrice := c.PostForm("cost_price"); costPrice != "" {
		product.CostPrice, _ = strconv.ParseFloat(costPrice, 64)
	}
	if promoPrice := c.PostForm("promotion_price"); promoPrice != "" {
		price, _ := strconv.ParseFloat(promoPrice, 64)
		product.PromotionPrice = &price
	}

	// Parse date fields (format: YYYY-MM-DD)
	if promoStart := c.PostForm("promotion_start"); promoStart != "" {
		if parsedTime, err := time.Parse("2006-01-02", promoStart); err == nil {
			product.PromotionStart = &parsedTime
		} else {
			log.Printf("Failed to parse promotion_start: %s, error: %v", promoStart, err)
		}
	}

	if promoEnd := c.PostForm("promotion_end"); promoEnd != "" {
		if parsedTime, err := time.Parse("2006-01-02", promoEnd); err == nil {
			product.PromotionEnd = &parsedTime
		} else {
			log.Printf("Failed to parse promotion_end: %s, error: %v", promoEnd, err)
		}
	}

	if expiryDate := c.PostForm("expiry_date"); expiryDate != "" {
		if parsedTime, err := time.Parse("2006-01-02", expiryDate); err == nil {
			product.ExpiryDate = &parsedTime
		} else {
			log.Printf("Failed to parse expiry_date: %s, error: %v", expiryDate, err)
		}
	}

	product.GrossMargin, _ = strconv.ParseFloat(c.PostForm("gross_margin"), 64)
	product.StaffDiscount, _ = strconv.ParseFloat(c.PostForm("staff_discount"), 64)
	product.TaxRate, _ = strconv.ParseFloat(c.PostForm("tax_rate"), 64)

	// Inventory
	if stock := c.PostForm("stock_quantity"); stock != "" {
		product.StockQuantity, _ = strconv.Atoi(stock)
	}
	product.ReorderLevel, _ = strconv.Atoi(c.PostForm("reorder_level"))
	product.ShelfLocation = c.PostForm("shelf_location")
	product.WeightVolume, _ = strconv.ParseFloat(c.PostForm("weight_volume"), 64)
	product.UnitOfMeasure = c.PostForm("unit_of_measure")

	// Dietary flags
	product.IsVegan = c.PostForm("is_vegan") == "true"
	product.IsGlutenFree = c.PostForm("is_gluten_free") == "true"
	product.IsVegetarian = c.PostForm("is_vegetarian") == "true"
	product.IsAgeRestricted = c.PostForm("is_age_restricted") == "true"

	if minAge := c.PostForm("minimum_age"); minAge != "" {
		age, _ := strconv.Atoi(minAge)
		product.MinimumAge = &age
	}

	// Additional Info
	product.Supplier = c.PostForm("supplier")
	product.CountryOfOrigin = c.PostForm("country_of_origin")
	product.AllergenInfo = c.PostForm("allergen_info")
	product.StorageType = c.PostForm("storage_type")
	product.IsOwnBrand = c.PostForm("is_own_brand") == "true"
	product.OnlineVisible = c.PostForm("online_visible") == "true"
	product.Status = c.PostForm("status")
	product.Notes = c.PostForm("notes")
	product.Barcode = c.PostForm("barcode")
	product.BatchNumber = c.PostForm("batch_number")

	// Category
	if categoryID := c.PostForm("category_id"); categoryID != "" {
		newCategoryID, err := uuid.Parse(categoryID)
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid category ID"})
			return
		}

		// Validate category exists
		if err := h.DB.First(&models.Category{}, "id = ?", newCategoryID).Error; err != nil {
			c.JSON(400, gin.H{"error": "Invalid category"})
			return
		}

		product.CategoryID = newCategoryID
	}

	// Update subcategory_id if provided
	if subcategoryID := c.PostForm("subcategory_id"); subcategoryID != "" {
		newSubcategoryID, err := uuid.Parse(subcategoryID)
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid subcategory ID"})
			return
		}

		// Validate subcategory exists
		if err := h.DB.First(&models.Subcategory{}, "id = ?", newSubcategoryID).Error; err != nil {
			c.JSON(400, gin.H{"error": "Invalid subcategory"})
			return
		}

		product.SubcategoryID = &newSubcategoryID
	} else {
		// If subcategory_id is not provided, set it to nil
		product.SubcategoryID = nil
	}

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
				objectPath, err := utils.ExtractObjectPath(productImage.ImageURL)
				if err == nil && objectPath != "" {
					if err := h.Storage.DeleteFile(objectPath); err != nil {
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
				// Validate file upload (content type + size)
				if err := utils.ValidateFileUpload(fileHeader); err != nil {
					c.JSON(400, gin.H{"error": err.Error()})
					return
				}

				file, err := fileHeader.Open()
				if err != nil {
					c.JSON(400, gin.H{"error": "Invalid image"})
					return
				}

				imageURL, err := h.Storage.UploadProductImage(
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

	// Handle franchise associations update
	if franchiseIDsStr := c.PostForm("franchise_ids"); franchiseIDsStr != "" {
		// Remove existing franchise links
		h.DB.Where("product_id = ?", product.ID).Delete(&models.FranchiseProduct{})

		// Create new franchise links
		franchiseIDs := strings.Split(franchiseIDsStr, ",")
		for _, fidStr := range franchiseIDs {
			fidStr = strings.TrimSpace(fidStr)
			if fidStr == "" {
				continue
			}
			parsedFID, err := uuid.Parse(fidStr)
			if err != nil {
				log.Printf("Invalid franchise ID in update product: %s", fidStr)
				continue
			}
			fp := models.FranchiseProduct{
				FranchiseID:   parsedFID,
				ProductID:     product.ID,
				StockQuantity: product.StockQuantity,
				ReorderLevel:  product.ReorderLevel,
				IsAvailable:   product.Status == "active",
			}
			if err := h.DB.Create(&fp).Error; err != nil {
				log.Printf("Failed to create franchise product link for franchise %s: %v", fidStr, err)
			}
		}
	}

	h.DB.Preload("Category").Preload("Subcategory").Preload("Images").First(&product, product.ID)
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
			objectPath, err := utils.ExtractObjectPath(productImage.ImageURL)
			if err == nil && objectPath != "" {
				if err := h.Storage.DeleteFile(objectPath); err != nil {
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

// GetProductFranchises returns the franchise IDs associated with a product
func (h *ProductHandler) GetProductFranchises(c *gin.Context) {
	id := c.Param("id")
	var franchiseProducts []models.FranchiseProduct
	if err := h.DB.Where("product_id = ?", id).Find(&franchiseProducts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch franchise associations"})
		return
	}

	franchiseIDs := make([]string, 0, len(franchiseProducts))
	for _, fp := range franchiseProducts {
		franchiseIDs = append(franchiseIDs, fp.FranchiseID.String())
	}
	c.JSON(http.StatusOK, gin.H{"franchise_ids": franchiseIDs})
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

	// Filter by franchise
	if franchiseID := c.Query("franchise_id"); franchiseID != "" {
		query = query.Where("id IN (SELECT product_id FROM franchise_products WHERE franchise_id = ?)", franchiseID)
	}

	// Search by name
	if search := c.Query("search"); search != "" {
		query = query.Where("LOWER(item_name) LIKE LOWER(?)", "%"+search+"%")
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
