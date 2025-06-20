package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"business-card-reader/internal/logger"
	"business-card-reader/internal/models"

	"encoding/base64"

	"github.com/google/uuid"
)

type BusinessCardService struct {
	dynamoService *DynamoService
	geminiService *GeminiService
	s3Service     *S3Service
}

func NewBusinessCardService(dynamoService *DynamoService, geminiService *GeminiService, s3Service *S3Service) *BusinessCardService {
	return &BusinessCardService{
		dynamoService: dynamoService,
		geminiService: geminiService,
		s3Service:     s3Service,
	}
}

// deepCopyBusinessCard creates a deep copy of BusinessCard without binary data
func (b *BusinessCardService) deepCopyBusinessCard(original *models.BusinessCard) *models.BusinessCard {
	copy := *original

	// Deep copy the Images slice
	copy.Images = make([]models.ImageData, len(original.Images))
	for i, img := range original.Images {
		copy.Images[i] = models.ImageData{
			FileName:    img.FileName,
			ContentType: img.ContentType,
			Size:        img.Size,
			S3Key:       img.S3Key,
			S3URL:       img.S3URL,
			Data:        nil, // Exclude binary data for DynamoDB
			Base64Data:  "",  // Exclude base64 data for DynamoDB
			UploadedAt:  img.UploadedAt,
		}
	}

	return &copy
}

func (b *BusinessCardService) ProcessBusinessCard(ctx context.Context, images []models.ImageUpload, observation string, user string) (*models.BusinessCard, error) {
	logger.LogInfo("ProcessBusinessCard", "Starting business card processing", map[string]interface{}{
		"image_count":        len(images),
		"has_observation":    observation != "",
		"observation_length": len(observation),
		"user":               user,
		"has_user":           user != "",
	})

	// Upload images to S3 and convert uploads to image data
	imageData := make([]models.ImageData, len(images))
	for i, upload := range images {
		logger.LogDebug("ProcessBusinessCard", "Processing image for S3 upload", map[string]interface{}{
			"index":        i,
			"file_name":    upload.FileName,
			"content_type": upload.ContentType,
			"size":         len(upload.Data),
		})

		// Upload image to S3
		s3Key, s3URL, err := b.s3Service.UploadImage(ctx, upload.Data, upload.FileName, upload.ContentType)
		if err != nil {
			logger.LogError("ProcessBusinessCard", err, map[string]interface{}{
				"step":      "s3_upload",
				"file_name": upload.FileName,
			})
			return nil, fmt.Errorf("failed to upload image %s to S3: %w", upload.FileName, err)
		}

		logger.LogInfo("ProcessBusinessCard", "Image uploaded to S3", map[string]interface{}{
			"file_name": upload.FileName,
			"s3_key":    s3Key,
			"s3_url":    s3URL,
		})

		imageData[i] = models.ImageData{
			FileName:    upload.FileName,
			ContentType: upload.ContentType,
			Size:        int64(len(upload.Data)),
			S3Key:       s3Key,
			S3URL:       s3URL,
			Data:        upload.Data, // Keep data for Gemini processing
			UploadedAt:  time.Now(),
		}

		// Log immediately after creation
		logger.LogInfo("ProcessBusinessCard", "ImageData created", map[string]interface{}{
			"index":          i,
			"file_name":      imageData[i].FileName,
			"original_size":  len(upload.Data),
			"stored_size":    len(imageData[i].Data),
			"data_preserved": len(imageData[i].Data) > 0,
		})
	}

	// Create initial business card record
	businessCardID := uuid.New().String()
	businessCard := &models.BusinessCard{
		ID:          businessCardID,
		Images:      imageData,
		Status:      models.StatusPending,
		Observation: observation,
		User:        user,
		CreatedAt:   time.Now(),
	}

	logger.LogInfo("ProcessBusinessCard", "Created business card record", map[string]interface{}{
		"business_card_id": businessCardID,
		"status":           models.StatusPending,
	})

	// Log image data in business card record
	for i, img := range businessCard.Images {
		logger.LogInfo("ProcessBusinessCard", "Image data in business card record", map[string]interface{}{
			"business_card_id": businessCardID,
			"image_index":      i,
			"file_name":        img.FileName,
			"data_size":        len(img.Data),
			"has_data":         len(img.Data) > 0,
		})
	}

	// Create deep copy without binary data for DynamoDB
	businessCardCopy := b.deepCopyBusinessCard(businessCard)

	// Save initial record
	err := b.dynamoService.SaveBusinessCard(ctx, businessCardCopy)
	if err != nil {
		logger.LogError("ProcessBusinessCard", err, map[string]interface{}{
			"step":             "save_initial_record",
			"business_card_id": businessCardID,
		})
		return nil, fmt.Errorf("failed to save initial business card: %w", err)
	}

	logger.LogInfo("ProcessBusinessCard", "Initial record saved to DynamoDB", map[string]interface{}{
		"business_card_id": businessCardID,
	})

	// Try to process with Gemini
	businessCard.Status = models.StatusProcessing

	// Create deep copy without binary data for DynamoDB
	businessCardCopy = b.deepCopyBusinessCard(businessCard)

	err = b.dynamoService.SaveBusinessCard(ctx, businessCardCopy)
	if err != nil {
		logger.LogError("ProcessBusinessCard", err, map[string]interface{}{
			"step":             "update_processing_status",
			"business_card_id": businessCardID,
		})
		return nil, fmt.Errorf("failed to update business card status: %w", err)
	}

	logger.LogInfo("ProcessBusinessCard", "Starting Gemini processing", map[string]interface{}{
		"business_card_id": businessCardID,
	})

	// Log image data before sending to Gemini
	for i, img := range imageData {
		logger.LogInfo("ProcessBusinessCard", "Image data before Gemini", map[string]interface{}{
			"business_card_id": businessCardID,
			"image_index":      i,
			"file_name":        img.FileName,
			"data_size":        len(img.Data),
			"has_data":         len(img.Data) > 0,
		})
	}

	// Extract data using Gemini
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

		// Create deep copy without binary data for DynamoDB
		businessCardCopy := b.deepCopyBusinessCard(businessCard)

		// Save failed state
		saveErr := b.dynamoService.SaveBusinessCard(ctx, businessCardCopy)
		if saveErr != nil {
			logger.LogError("ProcessBusinessCard", saveErr, map[string]interface{}{
				"step":             "save_failed_state",
				"business_card_id": businessCardID,
			})
			return nil, fmt.Errorf("failed to save error state: %w", saveErr)
		}

		logger.LogWarn("ProcessBusinessCard", "Business card marked as failed", map[string]interface{}{
			"business_card_id": businessCardID,
			"error":            err.Error(),
		})

		return businessCard, fmt.Errorf("failed to process business card: %w", err)
	}

	logger.LogInfo("ProcessBusinessCard", "Gemini processing completed", map[string]interface{}{
		"business_card_id": businessCardID,
	})

	// Update with processed data
	businessCard.PersonalData = processedCard.PersonalData
	businessCard.CompanyData = processedCard.CompanyData
	businessCard.ExtractedText = processedCard.ExtractedText
	businessCard.ProcessedAt = time.Now()
	businessCard.Status = models.StatusCompleted

	// Create deep copy without binary data for DynamoDB
	businessCardCopy = b.deepCopyBusinessCard(businessCard)

	// Save final state
	err = b.dynamoService.SaveBusinessCard(ctx, businessCardCopy)
	if err != nil {
		logger.LogError("ProcessBusinessCard", err, map[string]interface{}{
			"step":             "save_final_state",
			"business_card_id": businessCardID,
		})
		return nil, fmt.Errorf("failed to save processed business card: %w", err)
	}

	logger.LogInfo("ProcessBusinessCard", "Business card processing completed successfully", map[string]interface{}{
		"business_card_id": businessCardID,
		"status":           models.StatusCompleted,
	})

	return businessCard, nil
}

func (b *BusinessCardService) RetryFailedProcessing(ctx context.Context, id string) (*models.BusinessCard, error) {
	// Get the failed business card
	businessCard, err := b.dynamoService.GetBusinessCard(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get business card: %w", err)
	}

	if businessCard.Status != models.StatusFailed {
		return nil, fmt.Errorf("business card is not in failed state")
	}

	// Download images from S3 to retry processing
	for i := range businessCard.Images {
		if businessCard.Images[i].S3Key != "" {
			data, err := b.s3Service.GetImage(ctx, businessCard.Images[i].S3Key)
			if err != nil {
				return nil, fmt.Errorf("failed to download image from S3: %w", err)
			}
			businessCard.Images[i].Data = data
		}
	}

	// Update status to retrying
	businessCard.Status = models.StatusRetrying
	businessCard.RetryCount++
	now := time.Now()
	businessCard.LastRetryAt = &now

	// Save retry state
	err = b.dynamoService.SaveBusinessCard(ctx, businessCard)
	if err != nil {
		return nil, fmt.Errorf("failed to update retry state: %w", err)
	}

	// Try to process with Gemini again
	processedCard, err := b.geminiService.ExtractBusinessCardData(ctx, businessCard.Images)
	if err != nil {
		// Update with new error
		businessCard.Status = models.StatusFailed
		businessCard.Error = err.Error()

		// Clear binary data before saving
		for i := range businessCard.Images {
			businessCard.Images[i].Data = nil
		}

		// Save failed state
		saveErr := b.dynamoService.SaveBusinessCard(ctx, businessCard)
		if saveErr != nil {
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

	// Clear binary data before saving final state
	for i := range businessCard.Images {
		businessCard.Images[i].Data = nil
	}

	// Save final state
	err = b.dynamoService.SaveBusinessCard(ctx, businessCard)
	if err != nil {
		return nil, fmt.Errorf("failed to save processed business card: %w", err)
	}

	return businessCard, nil
}

func (b *BusinessCardService) GetBusinessCard(ctx context.Context, id string) (*models.BusinessCard, error) {
	return b.dynamoService.GetBusinessCard(ctx, id)
}

func (b *BusinessCardService) GetBusinessCardByIDWithImages(ctx context.Context, id string) (*models.BusinessCard, error) {
	logger.LogInfo("GetBusinessCardByIDWithImages", "Getting business card with images", map[string]interface{}{
		"business_card_id": id,
	})

	// Get business card from DynamoDB
	businessCard, err := b.dynamoService.GetBusinessCard(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get business card: %w", err)
	}

	// Download images from S3 and convert to base64
	for i := range businessCard.Images {
		if businessCard.Images[i].S3Key != "" {
			logger.LogInfo("GetBusinessCardByIDWithImages", "Attempting to download image from S3", map[string]interface{}{
				"business_card_id": id,
				"image_index":      i,
				"s3_key":           businessCard.Images[i].S3Key,
				"file_name":        businessCard.Images[i].FileName,
				"s3_url":           businessCard.Images[i].S3URL,
			})

			// Download image data from S3
			data, err := b.s3Service.GetImage(ctx, businessCard.Images[i].S3Key)
			if err != nil {
				// Categorize the error type for better debugging
				errorType := "unknown"
				if strings.Contains(err.Error(), "AccessDenied") {
					errorType = "access_denied"
				} else if strings.Contains(err.Error(), "NoSuchKey") {
					errorType = "file_not_found"
				} else if strings.Contains(err.Error(), "NoSuchBucket") {
					errorType = "bucket_not_found"
				}

				logger.LogError("GetBusinessCardByIDWithImages", err, map[string]interface{}{
					"business_card_id": id,
					"image_index":      i,
					"s3_key":           businessCard.Images[i].S3Key,
					"step":             "s3_download",
					"error_type":       errorType,
					"s3_url":           businessCard.Images[i].S3URL,
				})

				// Don't fail the entire request for one image - just log the error and continue
				businessCard.Images[i].Base64Data = ""
				businessCard.Images[i].Data = nil
				continue
			}

			// Convert to base64
			base64Data := base64.StdEncoding.EncodeToString(data)
			businessCard.Images[i].Base64Data = base64Data
			businessCard.Images[i].Data = data // Also include raw data

			logger.LogInfo("GetBusinessCardByIDWithImages", "Image downloaded and converted to base64", map[string]interface{}{
				"business_card_id": id,
				"image_index":      i,
				"file_name":        businessCard.Images[i].FileName,
				"data_size":        len(data),
				"base64_size":      len(base64Data),
				"success":          true,
			})
		} else {
			logger.LogWarn("GetBusinessCardByIDWithImages", "No S3 key found for image", map[string]interface{}{
				"business_card_id": id,
				"image_index":      i,
				"file_name":        businessCard.Images[i].FileName,
			})
		}
	}

	// Count successful downloads
	successfulDownloads := 0
	totalImages := len(businessCard.Images)
	for _, img := range businessCard.Images {
		if img.Base64Data != "" {
			successfulDownloads++
		}
	}

	logger.LogInfo("GetBusinessCardByIDWithImages", "Business card retrieval completed", map[string]interface{}{
		"business_card_id":     id,
		"total_images":         totalImages,
		"successful_downloads": successfulDownloads,
		"failed_downloads":     totalImages - successfulDownloads,
	})

	return businessCard, nil
}

func (b *BusinessCardService) GetAllBusinessCards(ctx context.Context) ([]models.BusinessCard, error) {
	return b.dynamoService.GetAllBusinessCards(ctx)
}

func (b *BusinessCardService) GetFailedBusinessCards(ctx context.Context) ([]models.BusinessCard, error) {
	return b.dynamoService.GetBusinessCardsByStatus(ctx, models.StatusFailed)
}

func (b *BusinessCardService) InitializeDatabase(ctx context.Context) error {
	return b.dynamoService.CreateTableIfNotExists(ctx)
}

func (b *BusinessCardService) UpdateObservation(ctx context.Context, id string, observation string) (*models.BusinessCard, error) {
	logger.LogInfo("UpdateObservation", "Updating business card observation", map[string]interface{}{
		"business_card_id":   id,
		"observation_length": len(observation),
	})

	// Get the existing business card
	businessCard, err := b.dynamoService.GetBusinessCard(ctx, id)
	if err != nil {
		logger.LogError("UpdateObservation", err, map[string]interface{}{
			"business_card_id": id,
			"step":             "get_business_card",
		})
		return nil, fmt.Errorf("failed to get business card: %w", err)
	}

	// Update the observation
	businessCard.Observation = observation

	// Save the updated business card
	err = b.dynamoService.SaveBusinessCard(ctx, businessCard)
	if err != nil {
		logger.LogError("UpdateObservation", err, map[string]interface{}{
			"business_card_id": id,
			"step":             "save_business_card",
		})
		return nil, fmt.Errorf("failed to save updated business card: %w", err)
	}

	logger.LogInfo("UpdateObservation", "Business card observation updated successfully", map[string]interface{}{
		"business_card_id": id,
	})

	return businessCard, nil
}
