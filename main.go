package main

import (
	"log"
	"os"
	"strings"

	"business-card-reader/docs"
	"business-card-reader/internal/config"
	"business-card-reader/internal/handlers"
	"business-card-reader/internal/logger"
	"business-card-reader/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title Business Card Reader API
// @version 1.0
// @description API for processing business cards using Gemini AI
// @host localhost:8080
// @BasePath /api/v1
func main() {
	// Initialize logger first
	logger.Init()

	// Load .env file
	_ = godotenv.Load()

	logger.LogInfo("main", "Application starting", map[string]interface{}{
		"version": "1.0.0",
	})

	// Log environment variables for debugging (mask sensitive values)
	log.Println("Loaded environment variables:")
	for _, key := range []string{
		"GEMINI_API_KEY", "GEMINI_MODEL_NAME", "AWS_REGION", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "DYNAMODB_TABLE_NAME", "S3_BUCKET_NAME", "S3_REGION", "PORT", "GIN_MODE", "AWS_ENDPOINT_URL"} {
		val := os.Getenv(key)
		if val == "" {
			log.Printf("  %s=NOT SET", key)
			continue
		}
		if strings.Contains(strings.ToLower(key), "key") || strings.Contains(strings.ToLower(key), "secret") {
			if len(val) > 6 {
				val = val[:2] + strings.Repeat("*", len(val)-4) + val[len(val)-2:]
			} else {
				val = "****"
			}
		}
		log.Printf("  %s=%s", key, val)
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load configuration:", err)
	}

	// Initialize services
	dynamoService, err := services.NewDynamoService(cfg.AWS.Region)
	if err != nil {
		log.Fatal("Failed to initialize DynamoDB service:", err)
	}

	s3Service, err := services.NewS3Service(cfg.S3.Region, cfg.S3.BucketName)
	if err != nil {
		log.Fatal("Failed to initialize S3 service:", err)
	}

	geminiService, err := services.NewGeminiService(cfg.Gemini.APIKey, cfg.Gemini.ModelName)
	if err != nil {
		log.Fatal("Failed to initialize Gemini service:", err)
	}

	businessCardService := services.NewBusinessCardService(dynamoService, geminiService, s3Service)

	// Initialize handlers
	handler := handlers.NewBusinessCardHandler(businessCardService)

	// Setup router
	router := gin.Default()

	// Add CORS middleware
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, access_token")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Initialize Swagger
	docs.SwaggerInfo.Title = "Business Card Reader API"
	docs.SwaggerInfo.Description = "API for processing business cards using Gemini AI"
	docs.SwaggerInfo.Version = "1.0"
	docs.SwaggerInfo.Host = "localhost:8080"
	docs.SwaggerInfo.BasePath = "/api/v1"
	docs.SwaggerInfo.Schemes = []string{"http"}

	// Add Swagger endpoint
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Routes
	api := router.Group("/api/v1")
	{
		api.POST("/business-cards", handler.ProcessBusinessCard)
		api.GET("/business-cards", handler.GetBusinessCards)
		api.GET("/business-cards/:id", handler.GetBusinessCardByID)
		api.PUT("/business-cards/:id/observation", handler.UpdateObservation)
		api.POST("/business-cards/:id/retry", handler.RetryFailedBusinessCard)
		api.GET("/business-cards/failed", handler.GetFailedBusinessCards)
	}

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy"})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	log.Fatal(router.Run(":" + port))
}
