package handlers

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"business-card-reader/internal/logger"
	"business-card-reader/internal/models"
	"business-card-reader/internal/services"

	"github.com/gin-gonic/gin"
)

type BusinessCardHandler struct {
	service *services.BusinessCardService
}

func NewBusinessCardHandler(service *services.BusinessCardService) *BusinessCardHandler {
	return &BusinessCardHandler{
		service: service,
	}
}

// @Summary Process business card images
// @Description Upload and process business card images using Gemini AI
// @Tags business-cards
// @Accept json
// @Produce json
// @Param request body models.BusinessCardRequestBase64 true "Business card images in base64 format"
// @Success 200 {object} models.BusinessCardResponse
// @Failure 400 {object} models.BusinessCardResponse
// @Failure 500 {object} models.BusinessCardResponse
// @Router /business-cards [post]
func (h *BusinessCardHandler) ProcessBusinessCard(c *gin.Context) {
	logger.LogInfo("ProcessBusinessCard", "Starting business card processing", map[string]interface{}{
		"user_agent":   c.GetHeader("User-Agent"),
		"remote_addr":  c.ClientIP(),
		"content_type": c.GetHeader("Content-Type"),
	})

	// Parse JSON request
	var request models.BusinessCardRequestBase64
	if err := c.ShouldBindJSON(&request); err != nil {
		logger.LogError("ProcessBusinessCard", err, map[string]interface{}{
			"step":        "parse_json_request",
			"remote_addr": c.ClientIP(),
		})
		c.JSON(http.StatusBadRequest, models.BusinessCardResponse{
			Success: false,
			Error:   "Invalid JSON request format",
		})
		return
	}

	if len(request.Images) == 0 {
		logger.LogWarn("ProcessBusinessCard", "No images provided", map[string]interface{}{
			"remote_addr": c.ClientIP(),
		})
		c.JSON(http.StatusBadRequest, models.BusinessCardResponse{
			Success: false,
			Error:   "At least one image is required",
		})
		return
	}

	if len(request.Images) > 2 {
		logger.LogWarn("ProcessBusinessCard", "Too many images provided", map[string]interface{}{
			"remote_addr": c.ClientIP(),
			"file_count":  len(request.Images),
		})
		c.JSON(http.StatusBadRequest, models.BusinessCardResponse{
			Success: false,
			Error:   "Maximum of 2 images allowed",
		})
		return
	}

	logger.LogInfo("ProcessBusinessCard", "Processing uploaded files", map[string]interface{}{
		"file_count":   len(request.Images),
		"timestamp":    request.Timestamp,
		"total_images": request.TotalImages,
	})

	var imageUploads []models.ImageUpload
	for i, imageBase64 := range request.Images {
		// Validate file type
		if !isValidImageType(imageBase64.ContentType) {
			logger.LogError("ProcessBusinessCard", fmt.Errorf("invalid file type: %s", imageBase64.ContentType), map[string]interface{}{
				"step":         "validate_file_type",
				"content_type": imageBase64.ContentType,
				"filename":     imageBase64.FileName,
				"file_index":   i,
			})
			c.JSON(http.StatusBadRequest, models.BusinessCardResponse{
				Success: false,
				Error:   fmt.Sprintf("Invalid file type: %s. Only JPEG, PNG, and WebP are allowed", imageBase64.ContentType),
			})
			return
		}

		// Decode base64 data
		data, err := base64.StdEncoding.DecodeString(imageBase64.Base64Data)
		if err != nil {
			logger.LogError("ProcessBusinessCard", err, map[string]interface{}{
				"step":       "decode_base64",
				"filename":   imageBase64.FileName,
				"file_index": i,
			})
			c.JSON(http.StatusBadRequest, models.BusinessCardResponse{
				Success: false,
				Error:   "Failed to decode base64 image data",
			})
			return
		}

		// Validate decoded size matches expected size
		if int64(len(data)) != imageBase64.Size {
			logger.LogWarn("ProcessBusinessCard", "Size mismatch between decoded data and expected size", map[string]interface{}{
				"filename":      imageBase64.FileName,
				"expected_size": imageBase64.Size,
				"actual_size":   len(data),
				"file_index":    i,
			})
		}

		logger.LogInfo("ProcessBusinessCard", "Image processed successfully", map[string]interface{}{
			"filename":      imageBase64.FileName,
			"file_size":     len(data),
			"expected_size": imageBase64.Size,
			"file_index":    i,
			"content_type":  imageBase64.ContentType,
		})

		imageUploads = append(imageUploads, models.ImageUpload{
			FileName:    imageBase64.FileName,
			ContentType: imageBase64.ContentType,
			Data:        data,
		})
	}

	// Process the business card
	businessCard, err := h.service.ProcessBusinessCard(c.Request.Context(), imageUploads)
	if err != nil {
		logger.LogError("ProcessBusinessCard", err, map[string]interface{}{
			"step":       "process_business_card",
			"file_count": len(imageUploads),
		})
		c.JSON(http.StatusInternalServerError, models.BusinessCardResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to process business card: %v", err),
		})
		return
	}

	logger.LogInfo("ProcessBusinessCard", "Business card processed successfully", map[string]interface{}{
		"business_card_id": businessCard.ID,
		"status":           businessCard.Status,
	})

	// Remove image data from response to keep it lightweight
	responseCard := *businessCard
	for i := range responseCard.Images {
		responseCard.Images[i].Data = nil
	}

	c.JSON(http.StatusOK, models.BusinessCardResponse{
		Success: true,
		Data:    responseCard,
	})
}

// @Summary Get all business cards
// @Description Retrieve all processed business cards
// @Tags business-cards
// @Produce json
// @Success 200 {object} models.BusinessCardListResponse
// @Failure 500 {object} models.BusinessCardListResponse
// @Router /business-cards [get]
func (h *BusinessCardHandler) GetBusinessCards(c *gin.Context) {
	logger.LogInfo("GetBusinessCards", "Retrieving all business cards", map[string]interface{}{
		"remote_addr": c.ClientIP(),
	})

	businessCards, err := h.service.GetAllBusinessCardsWithImages(c.Request.Context())
	if err != nil {
		logger.LogError("GetBusinessCards", err, map[string]interface{}{
			"step": "get_all_business_cards",
		})
		c.JSON(http.StatusInternalServerError, models.BusinessCardListResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to retrieve business cards: %v", err),
		})
		return
	}

	logger.LogInfo("GetBusinessCards", "Business cards retrieved successfully", map[string]interface{}{
		"count": len(businessCards),
	})

	// Remove sensitive data from response
	for i := range businessCards {
		for j := range businessCards[i].Images {
			businessCards[i].Images[j].Data = nil
		}
	}

	c.JSON(http.StatusOK, models.BusinessCardListResponse{
		Success: true,
		Data:    businessCards,
		Count:   len(businessCards),
	})
}

// @Summary Get business card by ID
// @Description Retrieve a specific business card by its ID
// @Tags business-cards
// @Produce json
// @Param id path string true "Business Card ID"
// @Success 200 {object} models.BusinessCardResponse
// @Failure 400 {object} models.BusinessCardResponse
// @Failure 404 {object} models.BusinessCardResponse
// @Router /business-cards/{id} [get]
func (h *BusinessCardHandler) GetBusinessCardByID(c *gin.Context) {
	id := c.Param("id")

	logger.LogInfo("GetBusinessCardByID", "Retrieving business card by ID", map[string]interface{}{
		"business_card_id": id,
		"remote_addr":      c.ClientIP(),
	})

	if id == "" {
		logger.LogWarn("GetBusinessCardByID", "No ID provided", map[string]interface{}{
			"remote_addr": c.ClientIP(),
		})
		c.JSON(http.StatusBadRequest, models.BusinessCardResponse{
			Success: false,
			Error:   "Business card ID is required",
		})
		return
	}

	businessCard, err := h.service.GetBusinessCardWithImages(c.Request.Context(), id)
	if err != nil {
		logger.LogError("GetBusinessCardByID", err, map[string]interface{}{
			"step":             "get_business_card",
			"business_card_id": id,
		})
		c.JSON(http.StatusNotFound, models.BusinessCardResponse{
			Success: false,
			Error:   fmt.Sprintf("Business card not found: %v", err),
		})
		return
	}

	logger.LogInfo("GetBusinessCardByID", "Business card retrieved successfully", map[string]interface{}{
		"business_card_id": id,
		"status":           businessCard.Status,
	})

	// Remove sensitive data from response
	for i := range businessCard.Images {
		businessCard.Images[i].Data = nil
	}

	c.JSON(http.StatusOK, models.BusinessCardResponse{
		Success: true,
		Data:    *businessCard,
	})
}

// isValidImageType checks if the content type is a valid image type
func isValidImageType(contentType string) bool {
	validTypes := []string{
		"image/jpeg",
		"image/jpg",
		"image/png",
		"image/webp",
	}

	contentType = strings.ToLower(contentType)
	for _, validType := range validTypes {
		if contentType == validType {
			return true
		}
	}
	return false
}

// @Summary Retry failed business card processing
// @Description Retry processing a failed business card
// @Tags business-cards
// @Produce json
// @Param id path string true "Business Card ID"
// @Success 200 {object} models.BusinessCardResponse
// @Failure 400 {object} models.BusinessCardResponse
// @Failure 500 {object} models.BusinessCardResponse
// @Router /business-cards/{id}/retry [post]
func (h *BusinessCardHandler) RetryFailedBusinessCard(c *gin.Context) {
	id := c.Param("id")

	logger.LogInfo("RetryFailedBusinessCard", "Starting retry for failed business card", map[string]interface{}{
		"business_card_id": id,
		"remote_addr":      c.ClientIP(),
	})

	if id == "" {
		logger.LogWarn("RetryFailedBusinessCard", "No ID provided", map[string]interface{}{
			"remote_addr": c.ClientIP(),
		})
		c.JSON(http.StatusBadRequest, models.BusinessCardResponse{
			Success: false,
			Error:   "Business card ID is required",
		})
		return
	}

	businessCard, err := h.service.RetryFailedProcessing(c.Request.Context(), id)
	if err != nil {
		logger.LogError("RetryFailedBusinessCard", err, map[string]interface{}{
			"step":             "retry_failed_processing",
			"business_card_id": id,
		})
		c.JSON(http.StatusInternalServerError, models.BusinessCardResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to retry processing: %v", err),
		})
		return
	}

	logger.LogInfo("RetryFailedBusinessCard", "Business card retry completed", map[string]interface{}{
		"business_card_id": id,
		"status":           businessCard.Status,
		"retry_count":      businessCard.RetryCount,
	})

	// Remove image data from response to keep it lightweight
	responseCard := *businessCard
	for i := range responseCard.Images {
		responseCard.Images[i].Data = nil
	}

	c.JSON(http.StatusOK, models.BusinessCardResponse{
		Success: true,
		Data:    responseCard,
	})
}

// @Summary Get failed business cards
// @Description Retrieve all failed business cards
// @Tags business-cards
// @Produce json
// @Success 200 {object} models.BusinessCardListResponse
// @Failure 500 {object} models.BusinessCardListResponse
// @Router /business-cards/failed [get]
func (h *BusinessCardHandler) GetFailedBusinessCards(c *gin.Context) {
	logger.LogInfo("GetFailedBusinessCards", "Retrieving failed business cards", map[string]interface{}{
		"remote_addr": c.ClientIP(),
	})

	businessCards, err := h.service.GetFailedBusinessCardsWithImages(c.Request.Context())
	if err != nil {
		logger.LogError("GetFailedBusinessCards", err, map[string]interface{}{
			"step": "get_failed_business_cards",
		})
		c.JSON(http.StatusInternalServerError, models.BusinessCardListResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to retrieve failed business cards: %v", err),
		})
		return
	}

	logger.LogInfo("GetFailedBusinessCards", "Failed business cards retrieved successfully", map[string]interface{}{
		"count": len(businessCards),
	})

	// Remove image data from response to keep it lightweight
	responseCards := make([]models.BusinessCard, len(businessCards))
	for i, card := range businessCards {
		responseCards[i] = card
		for j := range responseCards[i].Images {
			responseCards[i].Images[j].Data = nil
		}
	}

	c.JSON(http.StatusOK, models.BusinessCardListResponse{
		Success: true,
		Data:    responseCards,
		Count:   len(responseCards),
	})
}

// @Summary Delete business card
// @Description Delete a business card and its associated S3 images
// @Tags business-cards
// @Produce json
// @Param id path string true "Business Card ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /business-cards/{id} [delete]
func (h *BusinessCardHandler) DeleteBusinessCard(c *gin.Context) {
	id := c.Param("id")

	logger.LogInfo("DeleteBusinessCard", "Starting deletion of business card", map[string]interface{}{
		"business_card_id": id,
		"remote_addr":      c.ClientIP(),
	})

	if id == "" {
		logger.LogWarn("DeleteBusinessCard", "No ID provided", map[string]interface{}{
			"remote_addr": c.ClientIP(),
		})
		c.JSON(http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Business card ID is required",
		})
		return
	}

	err := h.service.DeleteBusinessCard(c.Request.Context(), id)
	if err != nil {
		logger.LogError("DeleteBusinessCard", err, map[string]interface{}{
			"step":             "delete_business_card",
			"business_card_id": id,
		})
		c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to delete business card: %v", err),
		})
		return
	}

	logger.LogInfo("DeleteBusinessCard", "Business card deleted successfully", map[string]interface{}{
		"business_card_id": id,
	})

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Business card deleted successfully",
	})
}
