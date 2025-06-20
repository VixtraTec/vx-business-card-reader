package handlers

import (
	"encoding/base64"
	"fmt"
	"io"
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
// @Description Upload and process business card images using Gemini AI. Supports both multipart/form-data and JSON with base64 images
// @Tags business-cards
// @Accept multipart/form-data
// @Accept json
// @Produce json
// @Param images formData file false "Business card images (max 2) - for multipart upload"
// @Param request body models.Base64BusinessCardRequest false "Business card images in base64 format - for JSON upload"
// @Success 200 {object} models.BusinessCardResponse
// @Failure 400 {object} models.BusinessCardResponse
// @Failure 500 {object} models.BusinessCardResponse
// @Router /business-cards [post]
func (h *BusinessCardHandler) ProcessBusinessCard(c *gin.Context) {
	contentType := c.GetHeader("Content-Type")

	logger.LogInfo("ProcessBusinessCard", "Starting business card processing", map[string]interface{}{
		"content_type": contentType,
		"client_ip":    c.ClientIP(),
		"user_agent":   c.GetHeader("User-Agent"),
	})

	// Check if it's JSON request with base64 data
	if strings.Contains(strings.ToLower(contentType), "application/json") {
		logger.LogDebug("ProcessBusinessCard", "Processing JSON request with base64", nil)
		h.processBusinessCardFromJSON(c)
		return
	}

	// Default to multipart form data processing
	logger.LogDebug("ProcessBusinessCard", "Processing multipart form data", nil)
	h.processBusinessCardFromMultipart(c)
}

// processBusinessCardFromJSON handles JSON requests with base64 images
func (h *BusinessCardHandler) processBusinessCardFromJSON(c *gin.Context) {
	var request models.Base64BusinessCardRequest

	logger.LogInfo("processBusinessCardFromJSON", "Processing JSON request", map[string]interface{}{
		"content_length": c.Request.ContentLength,
	})

	if err := c.ShouldBindJSON(&request); err != nil {
		logger.LogError("processBusinessCardFromJSON", err, map[string]interface{}{
			"step": "json_binding",
		})
		c.JSON(http.StatusBadRequest, models.BusinessCardResponse{
			Success: false,
			Error:   "Invalid JSON format: " + err.Error(),
		})
		return
	}

	logger.LogInfo("processBusinessCardFromJSON", "JSON parsed successfully", map[string]interface{}{
		"image_count":  len(request.Images),
		"total_images": request.TotalImages,
		"timestamp":    request.Timestamp,
	})

	// Validate request
	if len(request.Images) == 0 {
		logger.LogWarn("processBusinessCardFromJSON", "No images provided", nil)
		c.JSON(http.StatusBadRequest, models.BusinessCardResponse{
			Success: false,
			Error:   "At least one image is required",
		})
		return
	}

	if len(request.Images) > 2 {
		logger.LogWarn("processBusinessCardFromJSON", "Too many images", map[string]interface{}{
			"image_count": len(request.Images),
		})
		c.JSON(http.StatusBadRequest, models.BusinessCardResponse{
			Success: false,
			Error:   "Maximum of 2 images allowed",
		})
		return
	}

	if request.TotalImages != len(request.Images) {
		logger.LogWarn("processBusinessCardFromJSON", "Image count mismatch", map[string]interface{}{
			"total_images": request.TotalImages,
			"actual_count": len(request.Images),
		})
		c.JSON(http.StatusBadRequest, models.BusinessCardResponse{
			Success: false,
			Error:   "Total images count doesn't match the number of images provided",
		})
		return
	}

	var imageUploads []models.ImageUpload
	for i, img := range request.Images {
		logger.LogDebug("processBusinessCardFromJSON", "Processing image", map[string]interface{}{
			"index":        i,
			"file_name":    img.FileName,
			"content_type": img.ContentType,
			"size":         img.Size,
		})

		// Validate content type
		if !isValidImageType(img.ContentType) {
			logger.LogError("processBusinessCardFromJSON", fmt.Errorf("invalid content type"), map[string]interface{}{
				"content_type": img.ContentType,
				"file_name":    img.FileName,
			})
			c.JSON(http.StatusBadRequest, models.BusinessCardResponse{
				Success: false,
				Error:   fmt.Sprintf("Invalid file type: %s. Only JPEG, PNG, and WebP are allowed", img.ContentType),
			})
			return
		}

		// Decode base64 data
		data, err := base64.StdEncoding.DecodeString(img.Base64Data)
		if err != nil {
			logger.LogError("processBusinessCardFromJSON", err, map[string]interface{}{
				"file_name": img.FileName,
				"step":      "base64_decode",
			})
			c.JSON(http.StatusBadRequest, models.BusinessCardResponse{
				Success: false,
				Error:   fmt.Sprintf("Invalid base64 data for file %s: %v", img.FileName, err),
			})
			return
		}

		// Validate decoded size matches provided size
		if int64(len(data)) != img.Size {
			logger.LogError("processBusinessCardFromJSON", fmt.Errorf("size mismatch"), map[string]interface{}{
				"file_name":     img.FileName,
				"expected_size": img.Size,
				"actual_size":   len(data),
			})
			c.JSON(http.StatusBadRequest, models.BusinessCardResponse{
				Success: false,
				Error:   fmt.Sprintf("Decoded data size doesn't match provided size for file %s", img.FileName),
			})
			return
		}

		logger.LogInfo("processBusinessCardFromJSON", "Image processed successfully", map[string]interface{}{
			"file_name": img.FileName,
			"size":      len(data),
		})

		imageUploads = append(imageUploads, models.ImageUpload{
			FileName:    img.FileName,
			ContentType: img.ContentType,
			Data:        data,
		})
	}

	logger.LogInfo("processBusinessCardFromJSON", "Starting business card processing", map[string]interface{}{
		"image_count": len(imageUploads),
	})

	// Process the business card
	businessCard, err := h.service.ProcessBusinessCard(c.Request.Context(), imageUploads, request.Observation, request.User)
	if err != nil {
		logger.LogError("processBusinessCardFromJSON", err, map[string]interface{}{
			"step": "business_card_processing",
		})
		c.JSON(http.StatusInternalServerError, models.BusinessCardResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to process business card: %v", err),
		})
		return
	}

	logger.LogInfo("processBusinessCardFromJSON", "Business card processed successfully", map[string]interface{}{
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

// processBusinessCardFromMultipart handles multipart/form-data requests
func (h *BusinessCardHandler) processBusinessCardFromMultipart(c *gin.Context) {
	// Parse multipart form
	err := c.Request.ParseMultipartForm(10 << 20) // 10 MB max
	if err != nil {
		c.JSON(http.StatusBadRequest, models.BusinessCardResponse{
			Success: false,
			Error:   "Failed to parse form data",
		})
		return
	}

	files := c.Request.MultipartForm.File["images"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, models.BusinessCardResponse{
			Success: false,
			Error:   "At least one image is required",
		})
		return
	}

	if len(files) > 2 {
		c.JSON(http.StatusBadRequest, models.BusinessCardResponse{
			Success: false,
			Error:   "Maximum of 2 images allowed",
		})
		return
	}

	var imageUploads []models.ImageUpload
	for _, fileHeader := range files {
		// Validate file type
		if !isValidImageType(fileHeader.Header.Get("Content-Type")) {
			c.JSON(http.StatusBadRequest, models.BusinessCardResponse{
				Success: false,
				Error:   fmt.Sprintf("Invalid file type: %s. Only JPEG, PNG, and WebP are allowed", fileHeader.Header.Get("Content-Type")),
			})
			return
		}

		// Open and read file
		file, err := fileHeader.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.BusinessCardResponse{
				Success: false,
				Error:   "Failed to read uploaded file",
			})
			return
		}
		defer file.Close()

		data, err := io.ReadAll(file)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.BusinessCardResponse{
				Success: false,
				Error:   "Failed to read file content",
			})
			return
		}

		imageUploads = append(imageUploads, models.ImageUpload{
			FileName:    fileHeader.Filename,
			ContentType: fileHeader.Header.Get("Content-Type"),
			Data:        data,
		})
	}

	// Process the business card (no observation or user for multipart uploads)
	businessCard, err := h.service.ProcessBusinessCard(c.Request.Context(), imageUploads, "", "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.BusinessCardResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to process business card: %v", err),
		})
		return
	}

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
	businessCards, err := h.service.GetAllBusinessCards(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.BusinessCardListResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to retrieve business cards: %v", err),
		})
		return
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
	if id == "" {
		c.JSON(http.StatusBadRequest, models.BusinessCardResponse{
			Success: false,
			Error:   "Business card ID is required",
		})
		return
	}

	logger.LogInfo("GetBusinessCardByID", "Getting business card by ID", map[string]interface{}{
		"business_card_id": id,
	})

	businessCard, err := h.service.GetBusinessCardByIDWithImages(c.Request.Context(), id)
	if err != nil {
		logger.LogError("GetBusinessCardByID", err, map[string]interface{}{
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
		"image_count":      len(businessCard.Images),
	})

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
	if id == "" {
		c.JSON(http.StatusBadRequest, models.BusinessCardResponse{
			Success: false,
			Error:   "Business card ID is required",
		})
		return
	}

	businessCard, err := h.service.RetryFailedProcessing(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.BusinessCardResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to retry processing: %v", err),
		})
		return
	}

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
	businessCards, err := h.service.GetFailedBusinessCards(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.BusinessCardListResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to retrieve failed business cards: %v", err),
		})
		return
	}

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

// @Summary Update business card observation
// @Description Update the observation field of a business card
// @Tags business-cards
// @Accept json
// @Produce json
// @Param id path string true "Business Card ID"
// @Param request body models.UpdateObservationRequest true "Observation update request"
// @Success 200 {object} models.BusinessCardResponse
// @Failure 400 {object} models.BusinessCardResponse
// @Failure 404 {object} models.BusinessCardResponse
// @Failure 500 {object} models.BusinessCardResponse
// @Router /business-cards/{id}/observation [put]
func (h *BusinessCardHandler) UpdateObservation(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, models.BusinessCardResponse{
			Success: false,
			Error:   "Business card ID is required",
		})
		return
	}

	var request models.UpdateObservationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		logger.LogError("UpdateObservation", err, map[string]interface{}{
			"business_card_id": id,
			"step":             "json_binding",
		})
		c.JSON(http.StatusBadRequest, models.BusinessCardResponse{
			Success: false,
			Error:   "Invalid JSON format: " + err.Error(),
		})
		return
	}

	logger.LogInfo("UpdateObservation", "Updating business card observation", map[string]interface{}{
		"business_card_id":   id,
		"observation_length": len(request.Observation),
	})

	businessCard, err := h.service.UpdateObservation(c.Request.Context(), id, request.Observation)
	if err != nil {
		logger.LogError("UpdateObservation", err, map[string]interface{}{
			"business_card_id": id,
		})
		c.JSON(http.StatusNotFound, models.BusinessCardResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to update observation: %v", err),
		})
		return
	}

	logger.LogInfo("UpdateObservation", "Business card observation updated successfully", map[string]interface{}{
		"business_card_id": id,
	})

	c.JSON(http.StatusOK, models.BusinessCardResponse{
		Success: true,
		Data:    *businessCard,
	})
}
