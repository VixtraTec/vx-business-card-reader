package services

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"time"

	"business-card-reader/internal/logger"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

type S3Service struct {
	client     *s3.Client
	bucketName string
	region     string
}

type S3Object struct {
	Key         string
	ContentType string
	Size        int64
	Data        []byte
}

func NewS3Service() (*S3Service, error) {
	bucketName := "vx-src-api-test"
	region := "us-east-1"

	logger.LogInfo("NewS3Service", "Initializing S3 service", map[string]interface{}{
		"region": region,
		"bucket": bucketName,
	})

	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		logger.LogError("NewS3Service", err, map[string]interface{}{
			"step":   "load_config",
			"region": region,
		})
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(cfg)

	logger.LogInfo("NewS3Service", "S3 service initialized successfully", map[string]interface{}{
		"region": region,
		"bucket": bucketName,
	})

	return &S3Service{
		client:     client,
		bucketName: bucketName,
		region:     region,
	}, nil
}

func (s *S3Service) UploadImage(ctx context.Context, businessCardID, fileName, contentType string, data []byte) (string, error) {
	// Generate unique key for the image
	fileExt := filepath.Ext(fileName)
	if fileExt == "" {
		// Determine extension from content type
		switch contentType {
		case "image/jpeg", "image/jpg":
			fileExt = ".jpg"
		case "image/png":
			fileExt = ".png"
		case "image/webp":
			fileExt = ".webp"
		default:
			fileExt = ".jpg"
		}
	}

	// Create S3 key: business-cards/{businessCardID}/{uuid}{extension}
	imageID := uuid.New().String()
	key := fmt.Sprintf("business-cards/%s/%s%s", businessCardID, imageID, fileExt)

	logger.LogInfo("UploadImage", "Uploading image to S3", map[string]interface{}{
		"business_card_id": businessCardID,
		"key":              key,
		"content_type":     contentType,
		"size":             len(data),
		"filename":         fileName,
	})

	// Upload to S3
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucketName),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
		Metadata: map[string]string{
			"business-card-id":  businessCardID,
			"original-filename": fileName,
			"uploaded-at":       time.Now().UTC().Format(time.RFC3339),
		},
	})

	if err != nil {
		logger.LogError("UploadImage", err, map[string]interface{}{
			"step":             "put_object",
			"business_card_id": businessCardID,
			"key":              key,
		})
		return "", fmt.Errorf("failed to upload image to S3: %w", err)
	}

	logger.LogInfo("UploadImage", "Image uploaded successfully", map[string]interface{}{
		"business_card_id": businessCardID,
		"key":              key,
		"size":             len(data),
	})

	return key, nil
}

func (s *S3Service) GetImage(ctx context.Context, key string) (*S3Object, error) {
	logger.LogDebug("GetImage", "Retrieving image from S3", map[string]interface{}{
		"key": key,
	})

	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	})

	if err != nil {
		logger.LogError("GetImage", err, map[string]interface{}{
			"step": "get_object",
			"key":  key,
		})
		return nil, fmt.Errorf("failed to get image from S3: %w", err)
	}
	defer result.Body.Close()

	// Read the data
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(result.Body)
	if err != nil {
		logger.LogError("GetImage", err, map[string]interface{}{
			"step": "read_body",
			"key":  key,
		})
		return nil, fmt.Errorf("failed to read image data: %w", err)
	}

	contentType := "application/octet-stream"
	if result.ContentType != nil {
		contentType = *result.ContentType
	}

	size := int64(0)
	if result.ContentLength != nil {
		size = *result.ContentLength
	}

	logger.LogDebug("GetImage", "Image retrieved successfully", map[string]interface{}{
		"key":          key,
		"content_type": contentType,
		"size":         size,
	})

	return &S3Object{
		Key:         key,
		ContentType: contentType,
		Size:        size,
		Data:        buf.Bytes(),
	}, nil
}

func (s *S3Service) GeneratePresignedURL(ctx context.Context, key string, expiration time.Duration) (string, error) {
	logger.LogDebug("GeneratePresignedURL", "Generating presigned URL", map[string]interface{}{
		"key":        key,
		"expiration": expiration.String(),
	})

	presignClient := s3.NewPresignClient(s.client)

	request, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiration
	})

	if err != nil {
		logger.LogError("GeneratePresignedURL", err, map[string]interface{}{
			"step": "presign_get_object",
			"key":  key,
		})
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	logger.LogDebug("GeneratePresignedURL", "Presigned URL generated successfully", map[string]interface{}{
		"key": key,
		"url": request.URL,
	})

	return request.URL, nil
}

func (s *S3Service) DeleteImage(ctx context.Context, key string) error {
	logger.LogInfo("DeleteImage", "Deleting image from S3", map[string]interface{}{
		"key": key,
	})

	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	})

	if err != nil {
		logger.LogError("DeleteImage", err, map[string]interface{}{
			"step": "delete_object",
			"key":  key,
		})
		return fmt.Errorf("failed to delete image from S3: %w", err)
	}

	logger.LogInfo("DeleteImage", "Image deleted successfully", map[string]interface{}{
		"key": key,
	})

	return nil
}

func (s *S3Service) DeleteBusinessCardImages(ctx context.Context, businessCardID string) error {
	logger.LogInfo("DeleteBusinessCardImages", "Deleting all images for business card", map[string]interface{}{
		"business_card_id": businessCardID,
	})

	// List all objects with the business card prefix
	prefix := fmt.Sprintf("business-cards/%s/", businessCardID)

	listResult, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucketName),
		Prefix: aws.String(prefix),
	})

	if err != nil {
		logger.LogError("DeleteBusinessCardImages", err, map[string]interface{}{
			"step":             "list_objects",
			"business_card_id": businessCardID,
			"prefix":           prefix,
		})
		return fmt.Errorf("failed to list images: %w", err)
	}

	// Delete each object
	for _, object := range listResult.Contents {
		if object.Key != nil {
			err := s.DeleteImage(ctx, *object.Key)
			if err != nil {
				logger.LogWarn("DeleteBusinessCardImages", "Failed to delete individual image", map[string]interface{}{
					"business_card_id": businessCardID,
					"key":              *object.Key,
					"error":            err.Error(),
				})
			}
		}
	}

	logger.LogInfo("DeleteBusinessCardImages", "All images deleted for business card", map[string]interface{}{
		"business_card_id": businessCardID,
		"images_deleted":   len(listResult.Contents),
	})

	return nil
}

func (s *S3Service) GetImageURL(key string) string {
	// Generate a simple URL (this would be a public URL if bucket allows public access)
	// For private buckets, you'd want to generate presigned URLs
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.bucketName, s.region, key)
}
