package services

import (
	"context"
	"fmt"
	"time"

	"business-card-reader/internal/logger"
	"business-card-reader/internal/models"

	"github.com/google/uuid"
)

type BusinessCardService struct {
	dynamoService *DynamoService
	geminiService *GeminiService
	s3Service     *S3Service
}

func NewBusinessCardService(dynamoService *DynamoService, geminiService *GeminiService, s3Service *S3Service) *BusinessCardService {
	logger.LogInfo("NewBusinessCardService", "Business card service initialized", map[string]interface{}{})
	return &BusinessCardService{
		dynamoService: dynamoService,
		geminiService: geminiService,
		s3Service:     s3Service,
	}
}

func (b *BusinessCardService) ProcessBusinessCard(ctx context.Context, images []models.ImageUpload) (*models.BusinessCard, error) {
	businessCardID := uuid.New().String()

	logger.LogInfo("ProcessBusinessCard", "Starting business card processing", map[string]interface{}{
		"business_card_id": businessCardID,
		"image_count":      len(images),
	})

	// Upload images to S3 and create image data
	imageData := make([]models.ImageData, len(images))
	for i, upload := range images {
		// Upload to S3
		s3Key, err := b.s3Service.UploadImage(ctx, businessCardID, upload.FileName, upload.ContentType, upload.Data)
		if err != nil {
			logger.LogError("ProcessBusinessCard", err, map[string]interface{}{
				"step":             "upload_to_s3",
				"business_card_id": businessCardID,
				"image_index":      i,
				"filename":         upload.FileName,
			})
			return nil, fmt.Errorf("failed to upload image to S3: %w", err)
		}

		imageData[i] = models.ImageData{
			FileName:    upload.FileName,
			ContentType: upload.ContentType,
			Size:        int64(len(upload.Data)),
			S3Key:       s3Key,
			Data:        upload.Data, // Keep for Gemini processing
			UploadedAt:  time.Now(),
		}

		logger.LogDebug("ProcessBusinessCard", "Image uploaded to S3", map[string]interface{}{
			"business_card_id": businessCardID,
			"image_index":      i,
			"filename":         upload.FileName,
			"content_type":     upload.ContentType,
			"size":             len(upload.Data),
			"s3_key":           s3Key,
		})
	}

	// Create initial business card record
	businessCard := &models.BusinessCard{
		ID:        businessCardID,
		Images:    imageData,
		Status:    models.StatusPending,
		CreatedAt: time.Now(),
	}

	// Save initial record
	logger.LogInfo("ProcessBusinessCard", "Saving initial business card record", map[string]interface{}{
		"business_card_id": businessCardID,
		"status":           models.StatusPending,
	})

	err := b.dynamoService.SaveBusinessCard(ctx, businessCard)
	if err != nil {
		logger.LogError("ProcessBusinessCard", err, map[string]interface{}{
			"step":             "save_initial_record",
			"business_card_id": businessCardID,
		})
		return nil, fmt.Errorf("failed to save initial business card: %w", err)
	}

	// Try to process with Gemini
	businessCard.Status = models.StatusProcessing
	logger.LogInfo("ProcessBusinessCard", "Updating status to processing", map[string]interface{}{
		"business_card_id": businessCardID,
		"status":           models.StatusProcessing,
	})

	err = b.dynamoService.SaveBusinessCard(ctx, businessCard)
	if err != nil {
		logger.LogError("ProcessBusinessCard", err, map[string]interface{}{
			"step":             "update_status_processing",
			"business_card_id": businessCardID,
		})
		return nil, fmt.Errorf("failed to update business card status: %w", err)
	}

	// Extract data using Gemini
	logger.LogInfo("ProcessBusinessCard", "Starting Gemini AI processing", map[string]interface{}{
		"business_card_id": businessCardID,
	})

	processedCard, err := b.geminiService.ExtractBusinessCardData(ctx, imageData)
	if err != nil {
		logger.LogError("ProcessBusinessCard", err, map[string]interface{}{
			"step":             "gemini_processing",
			"business_card_id": businessCardID,
		})

		// Update card with error information
		businessCard.Status = models.StatusFailed
		businessCard.Error = err.Error()
		businessCard.RetryCount = 1
		now := time.Now()
		businessCard.LastRetryAt = &now

		logger.LogInfo("ProcessBusinessCard", "Updating status to failed", map[string]interface{}{
			"business_card_id": businessCardID,
			"status":           models.StatusFailed,
			"error":            err.Error(),
		})

		// Save failed state
		saveErr := b.dynamoService.SaveBusinessCard(ctx, businessCard)
		if saveErr != nil {
			logger.LogError("ProcessBusinessCard", saveErr, map[string]interface{}{
				"step":             "save_failed_state",
				"business_card_id": businessCardID,
			})
			return nil, fmt.Errorf("failed to save error state: %w", saveErr)
		}

		return businessCard, fmt.Errorf("failed to process business card: %w", err)
	}

	// Update with processed data
	businessCard.PersonalData = processedCard.PersonalData
	businessCard.CompanyData = processedCard.CompanyData
	businessCard.ExtractedText = processedCard.ExtractedText
	businessCard.ProcessedAt = time.Now()
	businessCard.Status = models.StatusCompleted

	logger.LogInfo("ProcessBusinessCard", "Business card processed successfully", map[string]interface{}{
		"business_card_id": businessCardID,
		"status":           models.StatusCompleted,
		"personal_name":    processedCard.PersonalData.FullName,
		"company_name":     processedCard.CompanyData.Name,
	})

	// Save final state
	err = b.dynamoService.SaveBusinessCard(ctx, businessCard)
	if err != nil {
		logger.LogError("ProcessBusinessCard", err, map[string]interface{}{
			"step":             "save_final_state",
			"business_card_id": businessCardID,
		})
		return nil, fmt.Errorf("failed to save processed business card: %w", err)
	}

	logger.LogInfo("ProcessBusinessCard", "Business card processing completed successfully", map[string]interface{}{
		"business_card_id": businessCardID,
	})

	return businessCard, nil
}

func (b *BusinessCardService) RetryFailedProcessing(ctx context.Context, id string) (*models.BusinessCard, error) {
	logger.LogInfo("RetryFailedProcessing", "Starting retry processing", map[string]interface{}{
		"business_card_id": id,
	})

	// Get the failed business card
	businessCard, err := b.dynamoService.GetBusinessCard(ctx, id)
	if err != nil {
		logger.LogError("RetryFailedProcessing", err, map[string]interface{}{
			"step":             "get_business_card",
			"business_card_id": id,
		})
		return nil, fmt.Errorf("failed to get business card: %w", err)
	}

	if businessCard.Status != models.StatusFailed {
		logger.LogWarn("RetryFailedProcessing", "Business card is not in failed state", map[string]interface{}{
			"business_card_id": id,
			"current_status":   businessCard.Status,
		})
		return nil, fmt.Errorf("business card is not in failed state")
	}

	// Update status to retrying
	businessCard.Status = models.StatusRetrying
	businessCard.RetryCount++
	now := time.Now()
	businessCard.LastRetryAt = &now

	logger.LogInfo("RetryFailedProcessing", "Updating status to retrying", map[string]interface{}{
		"business_card_id": id,
		"retry_count":      businessCard.RetryCount,
	})

	// Save retry state
	err = b.dynamoService.SaveBusinessCard(ctx, businessCard)
	if err != nil {
		logger.LogError("RetryFailedProcessing", err, map[string]interface{}{
			"step":             "save_retry_state",
			"business_card_id": id,
		})
		return nil, fmt.Errorf("failed to update retry state: %w", err)
	}

	// Prepare images for Gemini retry processing (download from S3)
	logger.LogInfo("RetryFailedProcessing", "Preparing images for Gemini AI retry processing", map[string]interface{}{
		"business_card_id": id,
		"retry_count":      businessCard.RetryCount,
	})

	geminiImages := make([]models.ImageData, len(businessCard.Images))
	for i, imgData := range businessCard.Images {
		// Download image from S3 for retry processing
		s3Object, err := b.s3Service.GetImage(ctx, imgData.S3Key)
		if err != nil {
			logger.LogError("RetryFailedProcessing", err, map[string]interface{}{
				"step":             "download_from_s3_for_retry",
				"business_card_id": id,
				"s3_key":           imgData.S3Key,
			})
			return nil, fmt.Errorf("failed to download image from S3 for retry processing: %w", err)
		}

		geminiImages[i] = models.ImageData{
			FileName:    imgData.FileName,
			ContentType: imgData.ContentType,
			Size:        imgData.Size,
			S3Key:       imgData.S3Key,
			Data:        s3Object.Data, // Only for Gemini processing
			UploadedAt:  imgData.UploadedAt,
		}
	}

	// Try to process with Gemini again
	logger.LogInfo("RetryFailedProcessing", "Starting Gemini AI retry processing", map[string]interface{}{
		"business_card_id": id,
		"retry_count":      businessCard.RetryCount,
	})

	processedCard, err := b.geminiService.ExtractBusinessCardData(ctx, geminiImages)
	if err != nil {
		logger.LogError("RetryFailedProcessing", err, map[string]interface{}{
			"step":             "gemini_retry_processing",
			"business_card_id": id,
			"retry_count":      businessCard.RetryCount,
		})

		// Update with new error
		businessCard.Status = models.StatusFailed
		businessCard.Error = err.Error()

		logger.LogInfo("RetryFailedProcessing", "Retry failed, updating status", map[string]interface{}{
			"business_card_id": id,
			"retry_count":      businessCard.RetryCount,
			"error":            err.Error(),
		})

		// Save failed state
		saveErr := b.dynamoService.SaveBusinessCard(ctx, businessCard)
		if saveErr != nil {
			logger.LogError("RetryFailedProcessing", saveErr, map[string]interface{}{
				"step":             "save_retry_failed_state",
				"business_card_id": id,
			})
			return nil, fmt.Errorf("failed to save error state: %w", saveErr)
		}

		return businessCard, fmt.Errorf("failed to process business card on retry: %w", err)
	}

	// Update with processed data
	businessCard.PersonalData = processedCard.PersonalData
	businessCard.CompanyData = processedCard.CompanyData
	businessCard.ExtractedText = processedCard.ExtractedText
	businessCard.ProcessedAt = time.Now()
	businessCard.Status = models.StatusCompleted
	businessCard.Error = "" // Clear any previous error

	logger.LogInfo("RetryFailedProcessing", "Retry processing completed successfully", map[string]interface{}{
		"business_card_id": id,
		"retry_count":      businessCard.RetryCount,
		"personal_name":    processedCard.PersonalData.FullName,
		"company_name":     processedCard.CompanyData.Name,
	})

	// Save final state
	err = b.dynamoService.SaveBusinessCard(ctx, businessCard)
	if err != nil {
		logger.LogError("RetryFailedProcessing", err, map[string]interface{}{
			"step":             "save_retry_final_state",
			"business_card_id": id,
		})
		return nil, fmt.Errorf("failed to save processed business card: %w", err)
	}

	return businessCard, nil
}

func (b *BusinessCardService) GetBusinessCard(ctx context.Context, id string) (*models.BusinessCard, error) {
	logger.LogDebug("GetBusinessCard", "Retrieving business card", map[string]interface{}{
		"business_card_id": id,
	})

	businessCard, err := b.dynamoService.GetBusinessCard(ctx, id)
	if err != nil {
		logger.LogError("GetBusinessCard", err, map[string]interface{}{
			"business_card_id": id,
		})
		return nil, err
	}

	return businessCard, nil
}

func (b *BusinessCardService) GetAllBusinessCards(ctx context.Context) ([]models.BusinessCard, error) {
	logger.LogDebug("GetAllBusinessCards", "Retrieving all business cards", map[string]interface{}{})

	businessCards, err := b.dynamoService.GetAllBusinessCards(ctx)
	if err != nil {
		logger.LogError("GetAllBusinessCards", err, map[string]interface{}{})
		return nil, err
	}

	logger.LogDebug("GetAllBusinessCards", "Retrieved business cards", map[string]interface{}{
		"count": len(businessCards),
	})

	return businessCards, nil
}

func (b *BusinessCardService) GetFailedBusinessCards(ctx context.Context) ([]models.BusinessCard, error) {
	logger.LogDebug("GetFailedBusinessCards", "Retrieving failed business cards", map[string]interface{}{})

	businessCards, err := b.dynamoService.GetBusinessCardsByStatus(ctx, models.StatusFailed)
	if err != nil {
		logger.LogError("GetFailedBusinessCards", err, map[string]interface{}{})
		return nil, err
	}

	logger.LogDebug("GetFailedBusinessCards", "Retrieved failed business cards", map[string]interface{}{
		"count": len(businessCards),
	})

	return businessCards, nil
}

func (b *BusinessCardService) InitializeDatabase(ctx context.Context) error {
	logger.LogInfo("InitializeDatabase", "Initializing database", map[string]interface{}{})

	err := b.dynamoService.CreateTableIfNotExists(ctx)
	if err != nil {
		logger.LogError("InitializeDatabase", err, map[string]interface{}{})
		return err
	}

	logger.LogInfo("InitializeDatabase", "Database initialized successfully", map[string]interface{}{})
	return nil
}

// GetBusinessCardWithImages retrieves a business card and populates image URLs
func (b *BusinessCardService) GetBusinessCardWithImages(ctx context.Context, id string) (*models.BusinessCard, error) {
	logger.LogDebug("GetBusinessCardWithImages", "Retrieving business card with images", map[string]interface{}{
		"business_card_id": id,
	})

	businessCard, err := b.dynamoService.GetBusinessCard(ctx, id)
	if err != nil {
		logger.LogError("GetBusinessCardWithImages", err, map[string]interface{}{
			"business_card_id": id,
		})
		return nil, err
	}

	// Generate presigned URLs for images (valid for 1 hour)
	for i := range businessCard.Images {
		if businessCard.Images[i].S3Key != "" {
			presignedURL, err := b.s3Service.GeneratePresignedURL(ctx, businessCard.Images[i].S3Key, time.Hour)
			if err != nil {
				logger.LogWarn("GetBusinessCardWithImages", "Failed to generate presigned URL", map[string]interface{}{
					"business_card_id": id,
					"s3_key":           businessCard.Images[i].S3Key,
					"error":            err.Error(),
				})
				// Continue without URL if presigning fails
				continue
			}
			businessCard.Images[i].S3URL = presignedURL
		}
	}

	return businessCard, nil
}

// GetAllBusinessCardsWithImages retrieves all business cards and populates image URLs
func (b *BusinessCardService) GetAllBusinessCardsWithImages(ctx context.Context) ([]models.BusinessCard, error) {
	logger.LogDebug("GetAllBusinessCardsWithImages", "Retrieving all business cards with images", map[string]interface{}{})

	businessCards, err := b.dynamoService.GetAllBusinessCards(ctx)
	if err != nil {
		logger.LogError("GetAllBusinessCardsWithImages", err, map[string]interface{}{})
		return nil, err
	}

	// Generate presigned URLs for all images
	for i := range businessCards {
		for j := range businessCards[i].Images {
			if businessCards[i].Images[j].S3Key != "" {
				presignedURL, err := b.s3Service.GeneratePresignedURL(ctx, businessCards[i].Images[j].S3Key, time.Hour)
				if err != nil {
					logger.LogWarn("GetAllBusinessCardsWithImages", "Failed to generate presigned URL", map[string]interface{}{
						"business_card_id": businessCards[i].ID,
						"s3_key":           businessCards[i].Images[j].S3Key,
						"error":            err.Error(),
					})
					// Continue without URL if presigning fails
					continue
				}
				businessCards[i].Images[j].S3URL = presignedURL
			}
		}
	}

	logger.LogDebug("GetAllBusinessCardsWithImages", "Retrieved business cards with images", map[string]interface{}{
		"count": len(businessCards),
	})

	return businessCards, nil
}

// GetFailedBusinessCardsWithImages retrieves failed business cards and populates image URLs
func (b *BusinessCardService) GetFailedBusinessCardsWithImages(ctx context.Context) ([]models.BusinessCard, error) {
	logger.LogDebug("GetFailedBusinessCardsWithImages", "Retrieving failed business cards with images", map[string]interface{}{})

	businessCards, err := b.dynamoService.GetBusinessCardsByStatus(ctx, models.StatusFailed)
	if err != nil {
		logger.LogError("GetFailedBusinessCardsWithImages", err, map[string]interface{}{})
		return nil, err
	}

	// Generate presigned URLs for all images
	for i := range businessCards {
		for j := range businessCards[i].Images {
			if businessCards[i].Images[j].S3Key != "" {
				presignedURL, err := b.s3Service.GeneratePresignedURL(ctx, businessCards[i].Images[j].S3Key, time.Hour)
				if err != nil {
					logger.LogWarn("GetFailedBusinessCardsWithImages", "Failed to generate presigned URL", map[string]interface{}{
						"business_card_id": businessCards[i].ID,
						"s3_key":           businessCards[i].Images[j].S3Key,
						"error":            err.Error(),
					})
					// Continue without URL if presigning fails
					continue
				}
				businessCards[i].Images[j].S3URL = presignedURL
			}
		}
	}

	logger.LogDebug("GetFailedBusinessCardsWithImages", "Retrieved failed business cards with images", map[string]interface{}{
		"count": len(businessCards),
	})

	return businessCards, nil
}

// DeleteBusinessCard deletes a business card and its associated S3 images
func (b *BusinessCardService) DeleteBusinessCard(ctx context.Context, id string) error {
	logger.LogInfo("DeleteBusinessCard", "Deleting business card and associated images", map[string]interface{}{
		"business_card_id": id,
	})

	// Get business card to retrieve S3 keys
	businessCard, err := b.dynamoService.GetBusinessCard(ctx, id)
	if err != nil {
		logger.LogError("DeleteBusinessCard", err, map[string]interface{}{
			"step":             "get_business_card",
			"business_card_id": id,
		})
		return fmt.Errorf("failed to get business card for deletion: %w", err)
	}

	// Log business card details for deletion tracking
	logger.LogDebug("DeleteBusinessCard", "Retrieved business card for deletion", map[string]interface{}{
		"business_card_id": businessCard.ID,
		"status":           businessCard.Status,
		"image_count":      len(businessCard.Images),
	})

	// Delete images from S3
	err = b.s3Service.DeleteBusinessCardImages(ctx, id)
	if err != nil {
		logger.LogWarn("DeleteBusinessCard", "Failed to delete S3 images, continuing with DB deletion", map[string]interface{}{
			"business_card_id": id,
			"error":            err.Error(),
		})
	}

	// Note: DynamoDB deletion would need to be implemented in DynamoService if needed
	logger.LogInfo("DeleteBusinessCard", "Business card deletion completed", map[string]interface{}{
		"business_card_id": id,
	})

	return nil
}
