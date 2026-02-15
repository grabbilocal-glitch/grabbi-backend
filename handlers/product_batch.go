package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"grabbi-backend/dtos"
	"grabbi-backend/firebase"
	"grabbi-backend/models"
	"grabbi-backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// parseImageURLs handles both array and newline/comma-separated string formats
func parseImageURLs(imageURLs interface{}) []string {
	var urls []string

	switch v := imageURLs.(type) {
	case []string:
		urls = v
	case string:
		// String that may contain newline or comma-separated URLs
		if v != "" {
			// Split by newlines and commas
			parts := strings.FieldsFunc(v, func(r rune) bool {
				return r == '\n' || r == '\r' || r == ','
			})

			for _, part := range parts {
				part = strings.TrimSpace(part)
				if part != "" {
					urls = append(urls, part)
				}
			}
		}
	case []interface{}:
		// JSON array from frontend
		for _, item := range v {
			if str, ok := item.(string); ok && str != "" {
				urls = append(urls, str)
			}
		}
	}

	return urls
}

// cleanURL standardizes URL by removing newlines, commas, and whitespace
func cleanURL(url string) string {
	url = strings.TrimSuffix(url, ",")
	url = strings.TrimSpace(url)
	url = strings.ReplaceAll(url, "\n", "")
	url = strings.ReplaceAll(url, "\r", "")
	return url
}

// uploadImagesConcurrently uploads multiple images concurrently with semaphore limit
func uploadImagesConcurrently(storage firebase.StorageClient, productID string, imageUrls []string) ([]models.ProductImage, []error) {
	const maxConcurrent = 3
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, maxConcurrent)

	type imageResult struct {
		index int
		url   string
		err   error
	}

	results := make(chan imageResult, len(imageUrls))

	for i, url := range imageUrls {
		wg.Add(1)
		semaphore <- struct{}{} // Acquire

		go func(idx int, imageURL string) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release

			firebaseURL, err := storage.DownloadAndUploadImage(imageURL, productID)
			results <- imageResult{
				index: idx,
				url:   firebaseURL,
				err:   err,
			}
		}(i, url)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results in order
	productImages := make([]models.ProductImage, len(imageUrls))
	errors := make([]error, len(imageUrls))
	for result := range results {
		if result.err == nil {
			productImages[result.index] = models.ProductImage{
				ProductID: uuid.MustParse(productID),
				ImageURL:  result.url,
				IsPrimary: result.index == 0,
			}
		} else {
			errors[result.index] = result.err
		}
	}

	return productImages, errors
}

// BatchImportProductsAsync handles bulk product import from Excel asynchronously
func (h *ProductHandler) BatchImportProducts(c *gin.Context) {
	var req dtos.ProductImportRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": utils.SanitizeValidationError(err)})
		return
	}

	// Create job
	job := utils.Store.CreateJob(len(req.Products))

	// Start background processing
	go h.processBatchImport(job, req.Products, req.DeleteMissing)

	// Return immediately with job ID
	c.JSON(http.StatusAccepted, gin.H{
		"job_id": job.ID.String(),
		"status": "processing",
		"total":  job.Total,
	})
}

// GetBatchJobStatus returns status of a batch import job
func (h *ProductHandler) GetBatchJobStatus(c *gin.Context) {
	id := c.Param("id")
	jobUUID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID"})
		return
	}

	job, exists := utils.Store.GetJob(jobUUID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	}

	c.JSON(http.StatusOK, job)
}

// processBatchImport processes products in background with concurrency and bulk operations
func (h *ProductHandler) processBatchImport(job *dtos.BatchJob, products []dtos.ProductImportItem, deleteMissing bool) {
	// Mark job as processing
	utils.Store.SetProcessing(job.ID)

	// Collect all product IDs from import
	importedProductIDs := make(map[uuid.UUID]bool)

	// Preload and cache all categories for batch operations
	categoryCache := make(map[uuid.UUID]*models.Category)
	categoryCacheMutex := sync.Mutex{}

	var allCategories []models.Category
	if err := h.DB.Find(&allCategories).Error; err != nil {
		log.Printf("Error loading categories: %v", err)
	} else {
		for i := range allCategories {
			categoryCache[allCategories[i].ID] = &allCategories[i]
		}
	}

	// Batch operation containers
	var productsToCreate []models.Product
	var productsToUpdate []models.Product
	var imagesToCreate []models.ProductImage
	batchMutex := sync.Mutex{}

	// Use worker pool pattern for concurrent product processing
	const maxConcurrentProducts = 5
	semaphore := make(chan struct{}, maxConcurrentProducts)
	var wg sync.WaitGroup
	totalProducts := len(products)
	processedCount := 0

	for i, productData := range products {
		wg.Add(1)
		semaphore <- struct{}{} // Acquire

		go func(idx int, data dtos.ProductImportItem) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release

			// Check if product is marked for deletion
			if data.Delete {
				// If ID is provided and delete flag is set, skip processing
				// The product will be deleted by deleteProductsNotInImport if not in import
				batchMutex.Lock()
				processedCount++
				progress := int(float64(processedCount) / float64(totalProducts) * 85)
				utils.Store.UpdateJob(job.ID, func(j *dtos.BatchJob) {
					j.Progress = progress
				})
				batchMutex.Unlock()
				return
			}

			// Prepare product (same logic as before, but don't save to DB yet)
			product, images, fieldsChanged, imagesChanged, err := h.prepareProductForBatch(
				data,
				categoryCache,
				&categoryCacheMutex,
			)

			if err != nil {
				log.Printf("Error preparing product %s: %v", data.ItemName, err)
				batchMutex.Lock()
				processedCount++
				progress := int(float64(processedCount) / float64(totalProducts) * 85)
				utils.Store.UpdateJob(job.ID, func(j *dtos.BatchJob) {
					j.Progress = progress
					j.Failed++
					j.Errors = append(j.Errors, dtos.JobError{
						Row:     idx + 2, // +2 for 1-indexed rows and header
						Product: data.ItemName,
						Fields:  map[string]string{"error": err.Error()},
					})
				})
				batchMutex.Unlock()
				return
			}

			// Collect product for batch insert/update
			batchMutex.Lock()
			if product.CreatedAt.IsZero() {
				// New product
				productsToCreate = append(productsToCreate, product)
				utils.Store.AddCreated(job.ID)
			} else {
				// Existing product
				productsToUpdate = append(productsToUpdate, product)
				// Only increment update counter if actual changes occurred
				if fieldsChanged || imagesChanged {
					utils.Store.AddUpdated(job.ID)
				}
			}

			// Collect images for batch insert
			imagesToCreate = append(imagesToCreate, images...)

			// Track product ID
			importedProductIDs[product.ID] = true

			// Update progress during processing phase (0-85%)
			processedCount++
			progress := int(float64(processedCount) / float64(totalProducts) * 85)
			utils.Store.UpdateJob(job.ID, func(j *dtos.BatchJob) {
				j.Progress = progress
			})

			batchMutex.Unlock()
		}(i, productData)
	}

	// Wait for all products to finish processing
	wg.Wait()

	utils.Store.UpdateJob(job.ID, func(j *dtos.BatchJob) {
		j.Progress = 85
	})

	if len(productsToCreate) > 0 {
		const batchSize = 100
		if err := h.DB.CreateInBatches(productsToCreate, batchSize).Error; err != nil {
			log.Printf("Error bulk creating products: %v", err)
		} else {
			log.Printf("Bulk created %d new products", len(productsToCreate))
		}
	}

	// Create FranchiseProduct entries for products with franchise_ids
	var franchiseProductsToCreate []models.FranchiseProduct
	for _, productData := range products {
		if len(productData.FranchiseIDs) == 0 {
			continue
		}
		// Find the product by SKU or name to get its ID
		var product models.Product
		if productData.SKU != "" {
			h.DB.Where("sku = ?", productData.SKU).First(&product)
		} else {
			h.DB.Where("item_name = ?", productData.ItemName).First(&product)
		}
		if product.ID == uuid.Nil {
			continue
		}
		for _, fidStr := range productData.FranchiseIDs {
			fidStr = strings.TrimSpace(fidStr)
			if fidStr == "" {
				continue
			}
			parsedFID, err := uuid.Parse(fidStr)
			if err != nil {
				log.Printf("Invalid franchise ID in batch import: %s", fidStr)
				continue
			}
			// Check if association already exists
			var existing models.FranchiseProduct
			if h.DB.Where("franchise_id = ? AND product_id = ?", parsedFID, product.ID).First(&existing).Error == nil {
				continue // Already exists
			}
			franchiseProductsToCreate = append(franchiseProductsToCreate, models.FranchiseProduct{
				FranchiseID:   parsedFID,
				ProductID:     product.ID,
				StockQuantity: productData.StockQuantity,
				ReorderLevel:  productData.ReorderLevel,
				IsAvailable:   productData.Status == "active",
			})
		}
	}
	if len(franchiseProductsToCreate) > 0 {
		if err := h.DB.CreateInBatches(franchiseProductsToCreate, 100).Error; err != nil {
			log.Printf("Error creating franchise product associations: %v", err)
		} else {
			log.Printf("Created %d franchise product associations", len(franchiseProductsToCreate))
		}
	}

	utils.Store.UpdateJob(job.ID, func(j *dtos.BatchJob) {
		j.Progress = 87
	})

	if len(productsToUpdate) > 0 {
		if err := h.DB.Save(&productsToUpdate).Error; err != nil {
			log.Printf("Error bulk updating products: %v", err)
		} else {
			log.Printf("Bulk updated %d products", len(productsToUpdate))
		}
	}

	utils.Store.UpdateJob(job.ID, func(j *dtos.BatchJob) {
		j.Progress = 89
	})

	if len(imagesToCreate) > 0 {
		const batchSize = 100
		if err := h.DB.CreateInBatches(imagesToCreate, batchSize).Error; err != nil {
			log.Printf("Error bulk creating images: %v", err)
		} else {
			log.Printf("Bulk created %d images", len(imagesToCreate))
		}
	}

	utils.Store.UpdateJob(job.ID, func(j *dtos.BatchJob) {
		j.Progress = 90
	})

	// Only delete products not in the import if explicitly requested
	if deleteMissing {
		h.deleteProductsNotInImport(job, importedProductIDs)
	} else {
		log.Println("Skipping deletion of products not in import (delete_missing=false)")
	}

	// Mark job as completed after all operations are done
	utils.Store.CompleteJob(job.ID, dtos.JobStatusCompleted)
}

// prepareProductForBatch prepares a product for bulk insert/update (doesn't save to DB)
func (h *ProductHandler) prepareProductForBatch(
	productData dtos.ProductImportItem,
	categoryCache map[uuid.UUID]*models.Category,
	categoryCacheMutex *sync.Mutex,
) (models.Product, []models.ProductImage, bool, bool, error) {
	var product models.Product
	var images []models.ProductImage
	fieldsChanged := false
	imagesChanged := false

	// Validate and get category from cache
	categoryUUID, err := uuid.Parse(productData.CategoryID)
	if err != nil {
		return product, nil, false, false, fmt.Errorf("invalid category ID format")
	}

	categoryCacheMutex.Lock()
	category, exists := categoryCache[categoryUUID]
	categoryCacheMutex.Unlock()

	if !exists {
		return product, nil, false, false, fmt.Errorf("category not found")
	}

	// Check if updating existing product or creating new
	if productData.ID != nil && *productData.ID != "" {
		// Use ID if provided in Excel
		productUUID, err := uuid.Parse(*productData.ID)
		if err != nil {
			return product, nil, false, false, fmt.Errorf("invalid product ID format")
		}

		// Check if product exists
		if err := h.DB.Where("id = ?", productUUID).First(&product).Error; err != nil {
			// Not found, create new
			product.ID = uuid.New()
		}
	} else {
		var existing models.Product
		if err := h.DB.Where("sku = ?", productData.SKU).First(&existing).Error; err == nil {
			product = existing
		} else {
			product.ID = uuid.New()
		}
	}

	// Auto-generate SKU if empty
	if productData.SKU == "" {
		generatedSKU, err := generateSKU(h.DB)
		if err != nil {
			return product, nil, false, false, fmt.Errorf("failed to generate SKU")
		}
		productData.SKU = generatedSKU
		log.Printf("Auto-generated SKU for product %s: %s", productData.ItemName, generatedSKU)
	}

	// Check if this is an update (product already exists)
	isUpdate := !product.CreatedAt.IsZero()

	// Store original values for comparison if updating
	var originalValues struct {
		Name           string
		Price          float64
		CategoryID     uuid.UUID
		Stock          int
		Brand          string
		PackSize       string
		Description    string
		IsVegan        bool
		IsGlutenFree   bool
		IsVegetarian   bool
		IsAgeRestricted bool
		MinimumAge     *int
		StorageType    string
		CostPrice      float64
		GrossMargin    float64
		StaffDiscount  float64
		TaxRate        float64
		ReorderLevel   int
		ShelfLocation  string
		WeightVolume   float64
		UnitOfMeasure  string
		Supplier       string
		CountryOfOrigin string
		AllergenInfo   string
		IsOwnBrand     bool
		OnlineVisible  bool
		Status         string
		Notes          string
		Barcode        string
		BatchNumber    string
		SubcategoryID  *uuid.UUID
	}

	if isUpdate {
		originalValues.Name = product.ItemName
		originalValues.Price = product.RetailPrice
		originalValues.CategoryID = product.CategoryID
		originalValues.Stock = product.StockQuantity
		originalValues.Brand = product.Brand
		originalValues.PackSize = product.PackSize
		originalValues.Description = product.ShortDescription
		originalValues.IsVegan = product.IsVegan
		originalValues.IsGlutenFree = product.IsGlutenFree
		originalValues.IsVegetarian = product.IsVegetarian
		originalValues.IsAgeRestricted = product.IsAgeRestricted
		originalValues.MinimumAge = product.MinimumAge
		originalValues.StorageType = product.StorageType
		originalValues.CostPrice = product.CostPrice
		originalValues.GrossMargin = product.GrossMargin
		originalValues.StaffDiscount = product.StaffDiscount
		originalValues.TaxRate = product.TaxRate
		originalValues.ReorderLevel = product.ReorderLevel
		originalValues.ShelfLocation = product.ShelfLocation
		originalValues.WeightVolume = product.WeightVolume
		originalValues.UnitOfMeasure = product.UnitOfMeasure
		originalValues.Supplier = product.Supplier
		originalValues.CountryOfOrigin = product.CountryOfOrigin
		originalValues.AllergenInfo = product.AllergenInfo
		originalValues.IsOwnBrand = product.IsOwnBrand
		originalValues.OnlineVisible = product.OnlineVisible
		originalValues.Status = product.Status
		originalValues.Notes = product.Notes
		originalValues.Barcode = product.Barcode
		originalValues.BatchNumber = product.BatchNumber
		originalValues.SubcategoryID = product.SubcategoryID
	}

	// Update product fields
	product.SKU = productData.SKU
	product.ItemName = productData.ItemName
	product.ShortDescription = productData.ShortDescription
	product.LongDescription = productData.LongDescription
	product.CostPrice = productData.CostPrice
	product.RetailPrice = productData.RetailPrice
	product.PromotionPrice = productData.PromotionPrice
	product.GrossMargin = productData.GrossMargin
	product.StaffDiscount = productData.StaffDiscount
	product.TaxRate = productData.TaxRate
	product.StockQuantity = productData.StockQuantity
	product.ReorderLevel = productData.ReorderLevel
	product.ShelfLocation = productData.ShelfLocation
	product.WeightVolume = productData.WeightVolume
	product.UnitOfMeasure = productData.UnitOfMeasure
	product.CategoryID = category.ID
	product.Brand = productData.Brand
	product.Supplier = productData.Supplier
	product.CountryOfOrigin = productData.CountryOfOrigin
	product.IsGlutenFree = productData.IsGlutenFree
	product.IsVegetarian = productData.IsVegetarian
	product.IsVegan = productData.IsVegan
	product.IsAgeRestricted = productData.IsAgeRestricted
	product.MinimumAge = productData.MinimumAge
	product.AllergenInfo = productData.AllergenInfo
	product.StorageType = productData.StorageType
	product.IsOwnBrand = productData.IsOwnBrand
	product.OnlineVisible = productData.OnlineVisible
	product.Status = productData.Status
	product.Barcode = productData.Barcode
	product.BatchNumber = productData.BatchNumber
	product.PackSize = productData.PackSize
	product.Notes = productData.Notes

		// Check if fields changed by comparing old vs new
	if isUpdate {
		fieldsChanged = (
			product.ItemName != originalValues.Name ||
			product.RetailPrice != originalValues.Price ||
			product.CategoryID != originalValues.CategoryID ||
			product.StockQuantity != originalValues.Stock ||
			product.Brand != originalValues.Brand ||
			product.PackSize != originalValues.PackSize ||
			product.ShortDescription != originalValues.Description ||
			product.IsVegan != originalValues.IsVegan ||
			product.IsGlutenFree != originalValues.IsGlutenFree ||
			product.IsVegetarian != originalValues.IsVegetarian ||
			product.IsAgeRestricted != originalValues.IsAgeRestricted ||
			product.MinimumAge != originalValues.MinimumAge ||
			product.StorageType != originalValues.StorageType ||
			product.CostPrice != originalValues.CostPrice ||
			product.GrossMargin != originalValues.GrossMargin ||
			product.StaffDiscount != originalValues.StaffDiscount ||
			product.TaxRate != originalValues.TaxRate ||
			product.ReorderLevel != originalValues.ReorderLevel ||
			product.ShelfLocation != originalValues.ShelfLocation ||
			product.WeightVolume != originalValues.WeightVolume ||
			product.UnitOfMeasure != originalValues.UnitOfMeasure ||
			product.Supplier != originalValues.Supplier ||
			product.CountryOfOrigin != originalValues.CountryOfOrigin ||
			product.AllergenInfo != originalValues.AllergenInfo ||
			product.IsOwnBrand != originalValues.IsOwnBrand ||
			product.OnlineVisible != originalValues.OnlineVisible ||
			product.Status != originalValues.Status ||
			product.Notes != originalValues.Notes ||
			product.Barcode != originalValues.Barcode ||
			product.BatchNumber != originalValues.BatchNumber ||
			(product.SubcategoryID == nil && originalValues.SubcategoryID != nil) ||
			(product.SubcategoryID != nil && originalValues.SubcategoryID == nil) ||
			(product.SubcategoryID != nil && originalValues.SubcategoryID != nil && *product.SubcategoryID != *originalValues.SubcategoryID))
	}

	// Handle optional date fields
	if productData.PromotionStart != "" {
		if parsedTime, err := time.Parse("2006-01-02", productData.PromotionStart); err == nil {
			product.PromotionStart = &parsedTime
		}
	}

	if productData.PromotionEnd != "" {
		if parsedTime, err := time.Parse("2006-01-02", productData.PromotionEnd); err == nil {
			product.PromotionEnd = &parsedTime
		}
	}

	if productData.ExpiryDate != "" {
		if parsedTime, err := time.Parse("2006-01-02", productData.ExpiryDate); err == nil {
			product.ExpiryDate = &parsedTime
		}
	}

	// Handle optional subcategory
	if productData.SubcategoryID != nil && *productData.SubcategoryID != "" {
		if subcategoryUUID, err := uuid.Parse(*productData.SubcategoryID); err == nil {
			product.SubcategoryID = &subcategoryUUID
		}
	}

	// Handle images - parse URLs which may be newline/comma-separated
	imageUrls := parseImageURLs(productData.ImageURLs)

	if len(imageUrls) > 0 {
		// Clean all URLs first
		cleanedURLs := make([]string, 0, len(imageUrls))
		for _, imageURL := range imageUrls {
			cleanedURL := cleanURL(imageURL)
			if cleanedURL != "" {
				cleanedURLs = append(cleanedURLs, cleanedURL)
			}
		}

		log.Printf("Product %s - After cleaning: %d URLs ready for upload", product.ItemName, len(cleanedURLs))

		if isUpdate {
			// Handle image updates for existing products
			if !productData.ImagesProvided {
				// Images not provided - keep existing images
				log.Printf("Product %s update - no images provided, preserving existing images", product.ItemName)
			} else {
				var existingImages []models.ProductImage
				if err := h.DB.Where("product_id = ?", product.ID).Find(&existingImages).Error; err == nil {
					existingURLMap := make(map[string]*models.ProductImage)
					for i := range existingImages {
						existingURLMap[existingImages[i].ImageURL] = &existingImages[i]
					}

					imagesToKeep := make(map[string]bool)
					imagesToDelete := make([]*models.ProductImage, 0)
					imagesToAdd := make([]string, 0)

					// Process new URLs
					for _, newURL := range cleanedURLs {
						if cleanedURL := cleanURL(newURL); cleanedURL != "" {
							if _, exists := existingURLMap[cleanedURL]; exists {
								imagesToKeep[cleanedURL] = true
								delete(existingURLMap, cleanedURL)
							} else {
								imagesToAdd = append(imagesToAdd, cleanedURL)
							}
						}
					}

					// Remaining in existingURLMap are images to delete
					for _, img := range existingURLMap {
						imagesToDelete = append(imagesToDelete, img)
					}

					// Log what will happen
					log.Printf("Product %s - Keep: %d, Delete: %d, Add: %d images",
						product.ItemName, len(imagesToKeep), len(imagesToDelete), len(imagesToAdd))

					// Mark as changed if we have additions or deletions
					if len(imagesToAdd) > 0 || len(imagesToDelete) > 0 {
						imagesChanged = true
						log.Printf("Product '%s' - Images changed: added=%d, removed=%d",
							product.ItemName, len(imagesToAdd), len(imagesToDelete))
					} else {
						log.Printf("Product '%s' - No image changes detected", product.ItemName)
					}

					// Delete images that are no longer in Excel
					if len(imagesToDelete) > 0 {
						for _, oldImage := range imagesToDelete {
							// Check if this image is referenced in any order
							var orderImageCount int64
							h.DB.Model(&models.OrderItem{}).
								Where("image_url = ?", oldImage.ImageURL).
								Count(&orderImageCount)

							if orderImageCount > 0 {
								log.Printf("Image %s is referenced in %d order(s) - preserving in Firebase storage",
									oldImage.ImageURL, orderImageCount)
							} else {
								// Image not used in any order - safe to delete from Firebase
								objectPath, _ := utils.ExtractObjectPath(oldImage.ImageURL)
								if objectPath != "" {
									if err := h.Storage.DeleteFile(objectPath); err != nil {
										if !strings.Contains(err.Error(), "object doesn't exist") {
											log.Printf("Failed to delete image from Firebase: %v", err)
										}
									}
								}
							}
							// Always delete product_image record from database
							h.DB.Delete(&oldImage)
							log.Printf("Product %s - Deleted image from DB: %s", product.ItemName, oldImage.ImageURL)
						}
					}

					// Process only new images to add
					cleanedURLs = imagesToAdd
				}
			}
		}

		// Only upload if there are URLs to process
		if len(cleanedURLs) > 0 {
			log.Printf("Product %s - Uploading %d images to Firebase", product.ItemName, len(cleanedURLs))

			// Upload images to Firebase and get Firebase URLs
			uploadResults, uploadErrors := uploadImagesConcurrently(h.Storage, product.ID.String(), cleanedURLs)

			// Log any errors
			for i, err := range uploadErrors {
				if err != nil {
					log.Printf("Failed to download/upload image %d for product %s: %v", i+1, product.ItemName, err)
				}
			}

			// Add successful uploads (these are Firebase URLs now)
			successCount := 0
			for _, img := range uploadResults {
				if img.ImageURL != "" {
					images = append(images, models.ProductImage{
						ProductID: product.ID,
						ImageURL:  img.ImageURL, // This is now a Firebase URL
						IsPrimary: img.IsPrimary,
					})
					successCount++
					log.Printf("Product %s - Successfully uploaded image to Firebase: %s (primary=%v)",
						product.ItemName, img.ImageURL, img.IsPrimary)
				}
			}
			log.Printf("Product %s - Total images to save to DB: %d out of %d attempted",
				product.ItemName, successCount, len(cleanedURLs))
		}
	} else if isUpdate && productData.ImagesProvided {
		// Images provided but empty - this means remove all existing images
		log.Printf("Product %s - Images provided but empty, removing all existing images", product.ItemName)
		
		var existingImages []models.ProductImage
		if err := h.DB.Where("product_id = ?", product.ID).Find(&existingImages).Error; err == nil {
			if len(existingImages) > 0 {
				imagesChanged = true
				for _, oldImage := range existingImages {
					// Check if this image is referenced in any order
					var orderImageCount int64
					h.DB.Model(&models.OrderItem{}).
						Where("image_url = ?", oldImage.ImageURL).
						Count(&orderImageCount)

					if orderImageCount > 0 {
						log.Printf("Image %s is referenced in %d order(s) - preserving in Firebase storage",
							oldImage.ImageURL, orderImageCount)
					} else {
						// Image not used in any order - safe to delete from Firebase
						objectPath, _ := utils.ExtractObjectPath(oldImage.ImageURL)
						if objectPath != "" {
							if err := h.Storage.DeleteFile(objectPath); err != nil {
								if !strings.Contains(err.Error(), "object doesn't exist") {
									log.Printf("Failed to delete image from Firebase: %v", err)
								}
							}
						}
					}
					// Always delete product_image record from database
					h.DB.Delete(&oldImage)
					log.Printf("Product %s - Deleted image from DB: %s", product.ItemName, oldImage.ImageURL)
				}
			}
		}
	}

	// Log final change status
	if isUpdate {
		changeTypes := []string{}
		if fieldsChanged {
			changeTypes = append(changeTypes, "fields")
		}
		if imagesChanged {
			changeTypes = append(changeTypes, "images")
		}
		if len(changeTypes) > 0 {
			log.Printf("Product '%s' - UPDATED (%s)", product.ItemName, strings.Join(changeTypes, ", "))
		} else {
			log.Printf("Product '%s' - No changes detected, NOT counting as update", product.ItemName)
		}
	}

	return product, images, fieldsChanged, imagesChanged, nil
}


// deleteProductsNotInImport deletes products that are not in the imported data
// This function safely handles deletion with order reference checks and manages progress
func (h *ProductHandler) deleteProductsNotInImport(job *dtos.BatchJob, importedProductIDs map[uuid.UUID]bool) {
	// Fetch all active products
	var allProducts []models.Product
	if err := h.DB.Where("deleted_at IS NULL").Find(&allProducts).Error; err != nil {
		log.Printf("Error fetching products for deletion check: %v", err)
		return
	}

	// Filter products not in import
	var productsToCheck []models.Product
	var productIDsToCheck []uuid.UUID

	for _, product := range allProducts {
		if !importedProductIDs[product.ID] {
			productsToCheck = append(productsToCheck, product)
			productIDsToCheck = append(productIDsToCheck, product.ID)
		}
	}

	if len(productIDsToCheck) == 0 {
		log.Printf("No products need to be checked for deletion")
		utils.Store.UpdateJob(job.ID, func(j *dtos.BatchJob) {
			j.Progress = 90
		})
		return
	}

	utils.Store.UpdateJob(job.ID, func(j *dtos.BatchJob) {
		j.Progress = 90
	})

	var orderCounts []dtos.ProductOrderCount
	if err := h.DB.Model(&models.OrderItem{}).
		Select("product_id as product_id, COUNT(*) as count").
		Where("product_id IN ?", productIDsToCheck).
		Group("product_id").
		Scan(&orderCounts).Error; err != nil {
		log.Printf("Error fetching order counts: %v", err)
		return
	}

	// Create a map for quick lookup
	orderCountMap := make(map[uuid.UUID]int64)
	for _, oc := range orderCounts {
		orderCountMap[oc.ProductID] = oc.Count
	}

	// Identify products safe to delete
	var safeToDelete []models.Product
	for _, product := range productsToCheck {
		if orderCountMap[product.ID] > 0 {
			log.Printf("Skipping deletion of product '%s' (ID: %s) - referenced in %d order(s)",
				product.ItemName, product.ID, orderCountMap[product.ID])
			continue
		}
		safeToDelete = append(safeToDelete, product)
	}

	deletedCount := 0
	skippedCount := len(productsToCheck) - len(safeToDelete)

	// Calculate progress increment per deleted product
	progressIncrement := 0
	if len(safeToDelete) > 0 {
		progressIncrement = 10 / len(safeToDelete)
	}

	h.deleteProductsBatch(safeToDelete)

	for _, product := range safeToDelete {
		log.Printf("Deleted product: %s (ID: %s)", product.ItemName, product.ID)
		deletedCount++
		utils.Store.AddDeleted(job.ID)

		if progressIncrement > 0 {
			utils.Store.UpdateJob(job.ID, func(j *dtos.BatchJob) {
				newProgress := 90 + (deletedCount * progressIncrement)
				if newProgress > 100 {
					newProgress = 100
				}
				j.Progress = newProgress
			})
		}
	}

	log.Printf("Batch import cleanup: Deleted %d products, skipped %d products (referenced in orders)",
		deletedCount, skippedCount)
}

// deleteProductsBatch deletes multiple products in optimized batch operations
func (h *ProductHandler) deleteProductsBatch(products []models.Product) {
	if len(products) == 0 {
		return
	}

	// Extract product IDs
	productIDs := make([]uuid.UUID, len(products))
	for i, p := range products {
		productIDs[i] = p.ID
	}

	// Batch 1: Fetch all images for these products at once
	var images []models.ProductImage
	if err := h.DB.Where("product_id IN ?", productIDs).Find(&images).Error; err != nil {
		log.Printf("Error fetching images for batch deletion: %v", err)
		return
	}

	// Extract image URLs
	imageURLs := make([]string, 0, len(images))
	for _, img := range images {
		imageURLs = append(imageURLs, img.ImageURL)
	}

	// Batch 2: Check all image URLs for order references at once
	var imageOrderCounts []dtos.ImageOrderCount
	if err := h.DB.Model(&models.OrderItem{}).
		Select("image_url as image_url, COUNT(*) as count").
		Where("image_url IN ?", imageURLs).
		Group("image_url").
		Scan(&imageOrderCounts).Error; err != nil {
		log.Printf("Error fetching image order counts: %v", err)
	}

	// Create map of images that are referenced in orders
	referencedImages := make(map[string]bool)
	for _, ioc := range imageOrderCounts {
		referencedImages[ioc.ImageURL] = true
	}

	// Batch 3: Delete images from Firebase that are not referenced (OPTIMIZED: Concurrent)
	const maxConcurrentDeletes = 5
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, maxConcurrentDeletes)

	// Filter images to delete (not referenced in orders)
	imagesToDelete := make([]models.ProductImage, 0)
	for _, img := range images {
		if !referencedImages[img.ImageURL] {
			imagesToDelete = append(imagesToDelete, img)
		}
	}

	if len(imagesToDelete) > 0 {
		log.Printf("Concurrently deleting %d images from Firebase", len(imagesToDelete))

		for _, img := range imagesToDelete {
			wg.Add(1)
			semaphore <- struct{}{} // Acquire

			go func(image models.ProductImage) {
				defer wg.Done()
				defer func() { <-semaphore }() // Release

				objectPath, _ := utils.ExtractObjectPath(image.ImageURL)
				if objectPath != "" {
					if err := h.Storage.DeleteFile(objectPath); err != nil {
						// Only log if not a 404 error
						if !strings.Contains(err.Error(), "object doesn't exist") {
							log.Printf("Failed to delete image %s from Firebase: %v", image.ImageURL, err)
						}
					} else {
						log.Printf("Successfully deleted image from Firebase: %s", image.ImageURL)
					}
				}
			}(img)
		}

		wg.Wait()
		log.Printf("Completed concurrent deletion of %d images", len(imagesToDelete))
	}

	// Batch 4: Soft delete all products at once
	if err := h.DB.Where("id IN ?", productIDs).Delete(&models.Product{}).Error; err != nil {
		log.Printf("Error batch deleting products: %v", err)
		return
	}
}
