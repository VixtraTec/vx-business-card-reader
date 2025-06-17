package services

import (
	"context"
	"fmt"
	"time"

	"business-card-reader/internal/models"

	"github.com/google/uuid"
)

type BusinessCardService struct {
	dynamoService *DynamoService
	geminiService *GeminiService
}

func NewBusinessCardService(dynamoService *DynamoService, geminiService *GeminiService) *BusinessCardService {
	return &BusinessCardService{
		dynamoService: dynamoService,
		geminiService: geminiService,
	}
}

func (b *BusinessCardService) ProcessBusinessCard(ctx context.Context, images []models.ImageUpload) (*models.BusinessCard, error) {
	// Convert uploads to image data
	imageData := make([]models.ImageData, len(images))
	for i, upload := range images {
		imageData[i] = models.ImageData{
			FileName:    upload.FileName,
			ContentType: upload.ContentType,
			Data:        upload.Data,
			Size:        int64(len(upload.Data)),
			UploadedAt:  time.Now(),
		}
	}

	// Create initial business card record
	businessCard := &models.BusinessCard{
		ID:        uuid.New().String(),
		Images:    imageData,
		Status:    models.StatusPending,
		CreatedAt: time.Now(),
	}

	// Save initial record
	err := b.dynamoService.SaveBusinessCard(ctx, businessCard)
	if err != nil {
		return nil, fmt.Errorf("failed to save initial business card: %w", err)
	}

	// Try to process with Gemini
	businessCard.Status = models.StatusProcessing
	err = b.dynamoService.SaveBusinessCard(ctx, businessCard)
	if err != nil {
		return nil, fmt.Errorf("failed to update business card status: %w", err)
	}

	// Extract data using Gemini
	processedCard, err := b.geminiService.ExtractBusinessCardData(ctx, imageData)
	if err != nil {
		// Update card with error information
		businessCard.Status = models.StatusFailed
		businessCard.Error = err.Error()
		businessCard.RetryCount = 1
		now := time.Now()
		businessCard.LastRetryAt = &now

		// Save failed state
		saveErr := b.dynamoService.SaveBusinessCard(ctx, businessCard)
		if saveErr != nil {
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

	// Save final state
	err = b.dynamoService.SaveBusinessCard(ctx, businessCard)
	if err != nil {
		return nil, fmt.Errorf("failed to save processed business card: %w", err)
	}

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

func (b *BusinessCardService) GetAllBusinessCards(ctx context.Context) ([]models.BusinessCard, error) {
	return b.dynamoService.GetAllBusinessCards(ctx)
}

func (b *BusinessCardService) GetFailedBusinessCards(ctx context.Context) ([]models.BusinessCard, error) {
	return b.dynamoService.GetBusinessCardsByStatus(ctx, models.StatusFailed)
}

func (b *BusinessCardService) InitializeDatabase(ctx context.Context) error {
	return b.dynamoService.CreateTableIfNotExists(ctx)
}
