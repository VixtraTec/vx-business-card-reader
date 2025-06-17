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
// @Description Upload and process business card images using Gemini AI
// @Tags business-cards
// @Accept multipart/form-data
// @Produce json
// @Param images formData file true "Business card images (max 2)"
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

	// Parse multipart form
	err := c.Request.ParseMultipartForm(10 << 20) // 10 MB max
	if err != nil {
		logger.LogError("ProcessBusinessCard", err, map[string]interface{}{
			"step":        "parse_multipart_form",
			"remote_addr": c.ClientIP(),
		})
		c.JSON(http.StatusBadRequest, models.BusinessCardResponse{
			Success: false,
			Error:   "Failed to parse form data",
		})
		return
	}

	files := c.Request.MultipartForm.File["images"]
	if len(files) == 0 {
		logger.LogWarn("ProcessBusinessCard", "No images provided", map[string]interface{}{
			"remote_addr": c.ClientIP(),
		})
		c.JSON(http.StatusBadRequest, models.BusinessCardResponse{
			Success: false,
			Error:   "At least one image is required",
		})
		return
	}

	if len(files) > 2 {
		logger.LogWarn("ProcessBusinessCard", "Too many images provided", map[string]interface{}{
			"remote_addr": c.ClientIP(),
			"file_count":  len(files),
		})
		c.JSON(http.StatusBadRequest, models.BusinessCardResponse{
			Success: false,
			Error:   "Maximum of 2 images allowed",
		})
		return
	}

	logger.LogInfo("ProcessBusinessCard", "Processing uploaded files", map[string]interface{}{
		"file_count": len(files),
	})

	var imageUploads []models.ImageUpload
	for i, fileHeader := range files {
		// Validate file type
		if !isValidImageType(fileHeader.Header.Get("Content-Type")) {
			logger.LogError("ProcessBusinessCard", fmt.Errorf("invalid file type: %s", fileHeader.Header.Get("Content-Type")), map[string]interface{}{
				"step":         "validate_file_type",
				"content_type": fileHeader.Header.Get("Content-Type"),
				"filename":     fileHeader.Filename,
				"file_index":   i,
			})
			c.JSON(http.StatusBadRequest, models.BusinessCardResponse{
				Success: false,
				Error:   fmt.Sprintf("Invalid file type: %s. Only JPEG, PNG, and WebP are allowed", fileHeader.Header.Get("Content-Type")),
			})
			return
		}

		// Open and read file
		file, err := fileHeader.Open()
		if err != nil {
			logger.LogError("ProcessBusinessCard", err, map[string]interface{}{
				"step":       "open_file",
				"filename":   fileHeader.Filename,
				"file_index": i,
			})
			c.JSON(http.StatusInternalServerError, models.BusinessCardResponse{
				Success: false,
				Error:   "Failed to read uploaded file",
			})
			return
		}
		defer file.Close()

		data, err := io.ReadAll(file)
		if err != nil {
			logger.LogError("ProcessBusinessCard", err, map[string]interface{}{
				"step":       "read_file_content",
				"filename":   fileHeader.Filename,
				"file_index": i,
			})
			c.JSON(http.StatusInternalServerError, models.BusinessCardResponse{
				Success: false,
				Error:   "Failed to read file content",
			})
			return
		}

		logger.LogInfo("ProcessBusinessCard", "File processed successfully", map[string]interface{}{
			"filename":   fileHeader.Filename,
			"file_size":  len(data),
			"file_index": i,
		})

		imageUploads = append(imageUploads, models.ImageUpload{
			FileName:    fileHeader.Filename,
			ContentType: fileHeader.Header.Get("Content-Type"),
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

	businessCards, err := h.service.GetAllBusinessCards(c.Request.Context())
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

	for i := range businessCards {
		for j := range businessCards[i].Images {
			if len(businessCards[i].Images[j].Data) > 0 {
				businessCards[i].Images[j].Base64Data = base64.StdEncoding.EncodeToString(businessCards[i].Images[j].Data)
				businessCards[i].Images[j].Data = nil
			}
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

	businessCard, err := h.service.GetBusinessCard(c.Request.Context(), id)
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

	for i := range businessCard.Images {
		if len(businessCard.Images[i].Data) > 0 {
			businessCard.Images[i].Base64Data = base64.StdEncoding.EncodeToString(businessCard.Images[i].Data)
			businessCard.Images[i].Data = nil
		}
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

	businessCards, err := h.service.GetFailedBusinessCards(c.Request.Context())
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
