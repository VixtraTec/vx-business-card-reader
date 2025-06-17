package models

import (
	"time"
)

// BusinessCard represents the complete business card data structure
type BusinessCard struct {
	ID            string       `json:"id" dynamodbav:"id"`
	PersonalData  PersonalData `json:"personal_data" dynamodbav:"personal_data"`
	CompanyData   CompanyData  `json:"company_data" dynamodbav:"company_data"`
	Images        []ImageData  `json:"images" dynamodbav:"images"`
	ExtractedText string       `json:"extracted_text" dynamodbav:"extracted_text"`
	ProcessedAt   time.Time    `json:"processed_at" dynamodbav:"processed_at"`
	CreatedAt     time.Time    `json:"created_at" dynamodbav:"created_at"`
	Status        string       `json:"status" dynamodbav:"status"`
	Error         string       `json:"error,omitempty" dynamodbav:"error,omitempty"`
	RetryCount    int          `json:"retry_count" dynamodbav:"retry_count"`
	LastRetryAt   *time.Time   `json:"last_retry_at,omitempty" dynamodbav:"last_retry_at,omitempty"`
}

// PersonalData contains personal information extracted from business card
type PersonalData struct {
	FullName   string `json:"full_name" dynamodbav:"full_name"`
	FirstName  string `json:"first_name" dynamodbav:"first_name"`
	LastName   string `json:"last_name" dynamodbav:"last_name"`
	JobTitle   string `json:"job_title" dynamodbav:"job_title"`
	Department string `json:"department" dynamodbav:"department"`
	Email      string `json:"email" dynamodbav:"email"`
	Phone      string `json:"phone" dynamodbav:"phone"`
	Mobile     string `json:"mobile" dynamodbav:"mobile"`
	LinkedIn   string `json:"linkedin" dynamodbav:"linkedin"`
	Website    string `json:"website" dynamodbav:"website"`
}

// CompanyData contains company information extracted from business card
type CompanyData struct {
	Name        string  `json:"name" dynamodbav:"name"`
	Industry    string  `json:"industry" dynamodbav:"industry"`
	Website     string  `json:"website" dynamodbav:"website"`
	Email       string  `json:"email" dynamodbav:"email"`
	Phone       string  `json:"phone" dynamodbav:"phone"`
	Address     Address `json:"address" dynamodbav:"address"`
	SocialMedia struct {
		LinkedIn  string `json:"linkedin" dynamodbav:"linkedin"`
		Twitter   string `json:"twitter" dynamodbav:"twitter"`
		Facebook  string `json:"facebook" dynamodbav:"facebook"`
		Instagram string `json:"instagram" dynamodbav:"instagram"`
	} `json:"social_media" dynamodbav:"social_media"`
}

// Address represents the company address
type Address struct {
	Street     string `json:"street" dynamodbav:"street"`
	City       string `json:"city" dynamodbav:"city"`
	State      string `json:"state" dynamodbav:"state"`
	PostalCode string `json:"postal_code" dynamodbav:"postal_code"`
	Country    string `json:"country" dynamodbav:"country"`
	Full       string `json:"full" dynamodbav:"full"`
}

// ImageData represents uploaded image information
type ImageData struct {
	FileName    string    `json:"file_name" dynamodbav:"file_name"`
	ContentType string    `json:"content_type" dynamodbav:"content_type"`
	Size        int64     `json:"size" dynamodbav:"size"`
	S3Key       string    `json:"s3_key" dynamodbav:"s3_key"`
	S3URL       string    `json:"s3_url,omitempty" dynamodbav:"-"`
	Data        []byte    `json:"-" dynamodbav:"-"`
	Base64Data  string    `json:"base64_data,omitempty" dynamodbav:"-"`
	UploadedAt  time.Time `json:"uploaded_at" dynamodbav:"uploaded_at"`
}

// BusinessCardRequest represents the request payload for processing business cards
type BusinessCardRequest struct {
	Images []ImageUpload `json:"images"`
}

// ImageUpload represents an uploaded image
type ImageUpload struct {
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	Data        []byte `json:"data"`
}

// ImageUploadBase64 represents an uploaded image with base64 data
type ImageUploadBase64 struct {
	FileName     string `json:"file_name"`
	ContentType  string `json:"content_type"`
	Size         int64  `json:"size"`
	Base64Data   string `json:"base64_data"`
	LastModified int64  `json:"last_modified"`
}

// BusinessCardRequestBase64 represents the new request payload with base64 images
type BusinessCardRequestBase64 struct {
	Images      []ImageUploadBase64 `json:"images"`
	Timestamp   string              `json:"timestamp"`
	TotalImages int                 `json:"total_images"`
}

// BusinessCardResponse represents the API response
type BusinessCardResponse struct {
	Success bool         `json:"success"`
	Data    BusinessCard `json:"data,omitempty"`
	Error   string       `json:"error,omitempty"`
}

// BusinessCardListResponse represents the list API response
type BusinessCardListResponse struct {
	Success bool           `json:"success"`
	Data    []BusinessCard `json:"data,omitempty"`
	Count   int            `json:"count"`
	Error   string         `json:"error,omitempty"`
}

// BusinessCardStatus represents the possible states of a business card
const (
	StatusPending    = "PENDING"
	StatusProcessing = "PROCESSING"
	StatusCompleted  = "COMPLETED"
	StatusFailed     = "FAILED"
	StatusRetrying   = "RETRYING"
)
