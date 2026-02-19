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

	// First, preload all franchises into a cache for efficient lookup
	var allFranchises []models.Franchise
	if err := h.DB.Find(&allFranchises).Error; err != nil {
		log.Printf("Error loading franchises for association: %v", err)
	}

	// Build franchise lookup map by name (name is now unique)
	franchiseByName := make(map[string]models.Franchise)
	for _, f := range allFranchises {
		franchiseByName[f.Name] = f
		log.Printf("Franchise cache: '%s' -> %s", f.Name, f.ID)
	}

	type franchiseAssoc struct {
		productID     uuid.UUID
		franchiseID   uuid.UUID
		stockQuantity int
		reorderLevel  int
		isAvailable   bool
	}
	var franchiseProductsToCreate []franchiseAssoc
	var franchiseIDsToRemove []struct {
		productID   uuid.UUID
		franchiseID uuid.UUID
	}

	for _, productData := range products {
		// Log what franchise data we received
		log.Printf("Product '%s' - FranchiseNames: '%s', FranchiseIDs: %v", productData.ItemName, productData.FranchiseNames, productData.FranchiseIDs)

		// Find the product by SKU or name to get its ID
		var product models.Product
		if productData.SKU != "" {
			h.DB.Where("sku = ?", productData.SKU).First(&product)
		} else {
			h.DB.Where("item_name = ?", productData.ItemName).First(&product)
		}
		if product.ID == uuid.Nil {
			log.Printf("Product '%s' not found in database, skipping franchise association", productData.ItemName)
			continue
		}

		// Check if franchise info was provided for this product
		hasFranchiseInfo := productData.FranchiseNames != "" || len(productData.FranchiseIDs) > 0

		// Collect desired franchise IDs from both FranchiseIDs and FranchiseNames
		desiredFranchiseIDs := make(map[uuid.UUID]bool)

		// Process FranchiseIDs (UUID strings)
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
			desiredFranchiseIDs[parsedFID] = true
			log.Printf("Product '%s' - Found FranchiseID: %s", productData.ItemName, parsedFID)
		}

		// Process FranchiseNames (newline-separated franchise names)
		if productData.FranchiseNames != "" {
			// Split by newlines
			nameLines := strings.Split(productData.FranchiseNames, "\n")
			log.Printf("Product '%s' - Parsed %d franchise name lines: %v", productData.ItemName, len(nameLines), nameLines)
			for _, nameLine := range nameLines {
				nameLine = strings.TrimSpace(nameLine)
				if nameLine == "" {
					continue
				}

				// Find franchise by name (name is unique)
				if franchise, ok := franchiseByName[nameLine]; ok {
					desiredFranchiseIDs[franchise.ID] = true
					log.Printf("Product '%s' - Found franchise by name '%s': %s", productData.ItemName, nameLine, franchise.ID)
					continue
				}

				log.Printf("Warning: Franchise '%s' not found for product '%s'", nameLine, productData.ItemName)
			}
		}

		log.Printf("Product '%s' - Desired franchise IDs: %v, hasFranchiseInfo: %v", productData.ItemName, desiredFranchiseIDs, hasFranchiseInfo)

		// Get existing franchise associations for this product
		var existingAssocs []models.FranchiseProduct
		h.DB.Where("product_id = ? AND deleted_at IS NULL", product.ID).Find(&existingAssocs)

		existingFranchiseIDs := make(map[uuid.UUID]bool)
		for _, ea := range existingAssocs {
			existingFranchiseIDs[ea.FranchiseID] = true
			log.Printf("Product '%s' - Existing franchise association: %s", productData.ItemName, ea.FranchiseID)
		}

		// If franchise info was provided, process additions and removals
		if hasFranchiseInfo {
			// Determine which associations to add
			for fid := range desiredFranchiseIDs {
				if !existingFranchiseIDs[fid] {
					// New association - add it
					franchiseProductsToCreate = append(franchiseProductsToCreate, franchiseAssoc{
						productID:     product.ID,
						franchiseID:   fid,
						stockQuantity: productData.StockQuantity,
						reorderLevel:  productData.ReorderLevel,
						isAvailable:   productData.Status == "active",
					})
					log.Printf("Adding franchise association: Product '%s' -> Franchise %s", productData.ItemName, fid)
				} else {
					log.Printf("Product '%s' - Franchise %s already associated, keeping", productData.ItemName, fid)
				}
			}

			// Determine which associations to remove
			for fid := range existingFranchiseIDs {
				if !desiredFranchiseIDs[fid] {
					// Association removed in import - mark for deletion
					franchiseIDsToRemove = append(franchiseIDsToRemove, struct {
						productID   uuid.UUID
						franchiseID uuid.UUID
					}{
						productID:   product.ID,
						franchiseID: fid,
					})
					log.Printf("Removing franchise association: Product '%s' -> Franchise %s", productData.ItemName, fid)
				}
			}
		}
	}

	// Remove franchise associations that are no longer in the import
	for _, remove := range franchiseIDsToRemove {
		// Soft delete the FranchiseProduct association
		now := time.Now()
		if err := h.DB.Model(&models.FranchiseProduct{}).
			Where("product_id = ? AND franchise_id = ? AND deleted_at IS NULL", remove.productID, remove.franchiseID).
			Update("deleted_at", now).Error; err != nil {
			log.Printf("Error removing franchise association: %v", err)
		} else {
			// Count as update for progress tracking
			utils.Store.AddUpdated(job.ID)
		}
	}

	// Create new franchise associations (or restore soft-deleted ones)
	if len(franchiseProductsToCreate) > 0 {
		var fpRecords []models.FranchiseProduct
		// Map to track which franchiseAssoc data to use for each restoration
		fpRecordsToRestore := make(map[uuid.UUID]franchiseAssoc) // key: FranchiseProduct.ID

		for _, fa := range franchiseProductsToCreate {
			// Check if there's a soft-deleted record for this franchise+product combination
			var softDeletedFP models.FranchiseProduct
			err := h.DB.Unscoped().
				Where("franchise_id = ? AND product_id = ? AND deleted_at IS NOT NULL", fa.franchiseID, fa.productID).
				First(&softDeletedFP).Error

			if err == nil {
				// Found a soft-deleted record - store it for restoration with new data
				fpRecordsToRestore[softDeletedFP.ID] = franchiseAssoc{
					productID:     fa.productID,
					franchiseID:   fa.franchiseID,
					stockQuantity: fa.stockQuantity,
					reorderLevel:  fa.reorderLevel,
					isAvailable:   fa.isAvailable,
				}
				log.Printf("Will restore soft-deleted franchise association: Product %s -> Franchise %s", fa.productID, fa.franchiseID)
			} else {
				// No soft-deleted record exists - create new
				fpRecords = append(fpRecords, models.FranchiseProduct{
					FranchiseID:   fa.franchiseID,
					ProductID:     fa.productID,
					StockQuantity: fa.stockQuantity,
					ReorderLevel:  fa.reorderLevel,
					IsAvailable:   fa.isAvailable,
				})
			}
		}

		// Restore soft-deleted records
		if len(fpRecordsToRestore) > 0 {
			now := time.Now()
			for fpID, fa := range fpRecordsToRestore {
				if err := h.DB.Model(&models.FranchiseProduct{}).
					Where("id = ?", fpID).
					Updates(map[string]interface{}{
						"deleted_at":     nil,
						"stock_quantity": fa.stockQuantity,
						"reorder_level":  fa.reorderLevel,
						"is_available":   fa.isAvailable,
						"updated_at":     now,
					}).Error; err != nil {
					log.Printf("Error restoring franchise product association %s: %v", fpID, err)
				} else {
					log.Printf("Restored soft-deleted franchise product association: Product %s -> Franchise %s", fa.productID, fa.franchiseID)
					// Count as update for progress tracking
					utils.Store.AddUpdated(job.ID)
				}
			}
		}

		// Create new records (for associations that never existed before)
		if len(fpRecords) > 0 {
			if err := h.DB.CreateInBatches(fpRecords, 100).Error; err != nil {
				log.Printf("Error creating franchise product associations: %v", err)
			} else {
				log.Printf("Created %d new franchise product associations", len(fpRecords))
				// Count each new franchise association as an update for progress tracking
				for range fpRecords {
					utils.Store.AddUpdated(job.ID)
				}
			}
		}

		if len(fpRecordsToRestore) > 0 || len(fpRecords) > 0 {
			log.Printf("Total franchise associations processed: %d restored, %d created", len(fpRecordsToRestore), len(fpRecords))
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
		// Verify which product IDs actually exist before inserting images
		// (products may have failed to create due to constraint violations)
		productIDSet := make(map[uuid.UUID]bool)
		for _, img := range imagesToCreate {
			productIDSet[img.ProductID] = true
		}
		productIDList := make([]uuid.UUID, 0, len(productIDSet))
		for id := range productIDSet {
			productIDList = append(productIDList, id)
		}

		var existingIDs []uuid.UUID
		h.DB.Model(&models.Product{}).Unscoped().Where("id IN ?", productIDList).Pluck("id", &existingIDs)
		existingIDSet := make(map[uuid.UUID]bool)
		for _, id := range existingIDs {
			existingIDSet[id] = true
		}

		validImages := make([]models.ProductImage, 0, len(imagesToCreate))
		for _, img := range imagesToCreate {
			if existingIDSet[img.ProductID] {
				validImages = append(validImages, img)
			} else {
				log.Printf("Skipping image for non-existent product %s: %s", img.ProductID, img.ImageURL)
			}
		}

		if len(validImages) > 0 {
			const batchSize = 100
			if err := h.DB.CreateInBatches(validImages, batchSize).Error; err != nil {
				log.Printf("Error bulk creating images: %v", err)
			} else {
				log.Printf("Bulk created %d images", len(validImages))
			}
		}
		if skipped := len(imagesToCreate) - len(validImages); skipped > 0 {
			log.Printf("Skipped %d images for non-existent products", skipped)
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
		Name            string
		Price           float64
		CategoryID      uuid.UUID
		Stock           int
		Brand           string
		PackSize        string
		Description     string
		IsVegan         bool
		IsGlutenFree    bool
		IsVegetarian    bool
		IsAgeRestricted bool
		MinimumAge      *int
		StorageType     string
		CostPrice       float64
		GrossMargin     float64
		StaffDiscount   float64
		TaxRate         float64
		ReorderLevel    int
		ShelfLocation   string
		WeightVolume    float64
		UnitOfMeasure   string
		Supplier        string
		CountryOfOrigin string
		AllergenInfo    string
		IsOwnBrand      bool
		OnlineVisible   bool
		Status          string
		Notes           string
		Barcode         *string
		BatchNumber     string
		SubcategoryID   *uuid.UUID
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
	if productData.Barcode != nil && *productData.Barcode == "" {
		product.Barcode = nil
	} else {
		product.Barcode = productData.Barcode
	}
	product.BatchNumber = productData.BatchNumber
	product.PackSize = productData.PackSize
	product.Notes = productData.Notes

	// Check if fields changed by comparing old vs new
	if isUpdate {
		fieldsChanged = (product.ItemName != originalValues.Name ||
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

// deleteProductsNotInImport soft-deletes products that are not in the imported data
// Products are soft-deleted regardless of order references - orders can still retrieve soft-deleted products
func (h *ProductHandler) deleteProductsNotInImport(job *dtos.BatchJob, importedProductIDs map[uuid.UUID]bool) {
	// Fetch all active products
	var allProducts []models.Product
	if err := h.DB.Where("deleted_at IS NULL").Find(&allProducts).Error; err != nil {
		log.Printf("Error fetching products for deletion check: %v", err)
		return
	}

	// Filter products not in import
	var productsToDelete []models.Product
	for _, product := range allProducts {
		if !importedProductIDs[product.ID] {
			productsToDelete = append(productsToDelete, product)
		}
	}

	if len(productsToDelete) == 0 {
		log.Printf("No products need to be deleted")
		utils.Store.UpdateJob(job.ID, func(j *dtos.BatchJob) {
			j.Progress = 90
		})
		return
	}

	utils.Store.UpdateJob(job.ID, func(j *dtos.BatchJob) {
		j.Progress = 90
	})

	// Calculate progress increment per deleted product
	progressIncrement := 0
	if len(productsToDelete) > 0 {
		progressIncrement = 10 / len(productsToDelete)
	}

	h.deleteProductsBatch(productsToDelete)

	deletedCount := 0
	for _, product := range productsToDelete {
		log.Printf("Soft-deleted product: %s (ID: %s)", product.ItemName, product.ID)
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

	log.Printf("Batch import cleanup: Soft-deleted %d products (preserved images and franchise associations)", deletedCount)

	// Verify the deleted count in job
	if job, exists := utils.Store.GetJob(job.ID); exists {
		log.Printf("DEBUG: Job %s - Deleted count after cleanup: %d", job.ID, job.Deleted)
	}
}

// deleteProductsBatch soft-deletes multiple products WITHOUT deleting images or franchise associations
// This allows franchise users to restore products in the franchise portal
func (h *ProductHandler) deleteProductsBatch(products []models.Product) {
	if len(products) == 0 {
		return
	}

	// Extract product IDs
	productIDs := make([]uuid.UUID, len(products))
	for i, p := range products {
		productIDs[i] = p.ID
	}

	// ONLY soft-delete the products - preserve images and franchise associations
	// This allows franchise portal to restore products with their images intact
	now := time.Now()
	if err := h.DB.Model(&models.Product{}).
		Where("id IN ?", productIDs).
		Update("deleted_at", now).Error; err != nil {
		log.Printf("Error batch soft-deleting products: %v", err)
		return
	}

	log.Printf("Soft-deleted %d products - images and franchise associations preserved for restoration", len(productIDs))
}
