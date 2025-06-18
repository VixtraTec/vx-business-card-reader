package services

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"business-card-reader/internal/logger"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/google/uuid"
)

type S3Service struct {
	client     *s3.S3
	bucketName string
	region     string
}

func NewS3Service(region, bucketName string) (*S3Service, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %w", err)
	}

	client := s3.New(sess)

	logger.LogInfo("S3Service", "Initialized S3 service", map[string]interface{}{
		"bucket": bucketName,
		"region": region,
		"sdk":    "v1",
	})

	return &S3Service{
		client:     client,
		bucketName: bucketName,
		region:     region,
	}, nil
}

// UploadImage uploads an image to S3 and returns the S3 key and URL
func (s *S3Service) UploadImage(ctx context.Context, data []byte, fileName, contentType string) (string, string, error) {
	// Generate unique S3 key
	timestamp := time.Now().Format("2006/01/02")
	fileExt := filepath.Ext(fileName)
	uniqueID := uuid.New().String()
	s3Key := fmt.Sprintf("business-cards/%s/%s%s", timestamp, uniqueID, fileExt)

	logger.LogInfo("S3UploadImage", "Starting S3 upload", map[string]interface{}{
		"file_name":    fileName,
		"content_type": contentType,
		"size":         len(data),
		"s3_key":       s3Key,
		"bucket":       s.bucketName,
		"sdk_version":  "v1",
	})

	// Upload to S3 using SDK v1
	_, err := s.client.PutObjectWithContext(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucketName),
		Key:         aws.String(s3Key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
		Metadata: map[string]*string{
			"original-filename": aws.String(fileName),
			"uploaded-at":       aws.String(time.Now().Format(time.RFC3339)),
		},
	})
	if err != nil {
		logger.LogError("S3UploadImage", err, map[string]interface{}{
			"file_name":   fileName,
			"s3_key":      s3Key,
			"bucket":      s.bucketName,
			"sdk_version": "v1",
		})
		return "", "", fmt.Errorf("failed to upload image to S3: %w", err)
	}

	// Generate S3 URL
	s3URL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.bucketName, s.region, s3Key)

	logger.LogInfo("S3UploadImage", "S3 upload completed successfully", map[string]interface{}{
		"file_name": fileName,
		"s3_key":    s3Key,
		"s3_url":    s3URL,
	})

	return s3Key, s3URL, nil
}

// GetImageURL returns the public URL for an S3 object
func (s *S3Service) GetImageURL(s3Key string) string {
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.bucketName, s.region, s3Key)
}

// GetImage downloads an image from S3 and returns the data
func (s *S3Service) GetImage(ctx context.Context, s3Key string) ([]byte, error) {
	logger.LogInfo("S3GetImage", "Starting S3 download", map[string]interface{}{
		"s3_key":      s3Key,
		"bucket":      s.bucketName,
		"sdk_version": "v1",
	})

	result, err := s.client.GetObjectWithContext(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(s3Key),
	})
	if err != nil {
		logger.LogError("S3GetImage", err, map[string]interface{}{
			"s3_key":      s3Key,
			"bucket":      s.bucketName,
			"sdk_version": "v1",
		})
		return nil, fmt.Errorf("failed to get image from S3: %w", err)
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		logger.LogError("S3GetImage", err, map[string]interface{}{
			"s3_key": s3Key,
			"step":   "read_body",
		})
		return nil, fmt.Errorf("failed to read image data: %w", err)
	}

	logger.LogInfo("S3GetImage", "S3 download completed successfully", map[string]interface{}{
		"s3_key": s3Key,
		"size":   len(data),
	})

	return data, nil
}

// DeleteImage deletes an image from S3
func (s *S3Service) DeleteImage(ctx context.Context, s3Key string) error {
	_, err := s.client.DeleteObjectWithContext(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(s3Key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete image from S3: %w", err)
	}
	return nil
}
