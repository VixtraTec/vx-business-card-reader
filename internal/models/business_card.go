package models

import (
	"time"
)

// BusinessCard represents the complete business card data structure
type BusinessCard struct {
	ID            string       `json:"id" dynamodb:"id"`
	PersonalData  PersonalData `json:"personal_data" dynamodb:"personal_data"`
	CompanyData   CompanyData  `json:"company_data" dynamodb:"company_data"`
	Images        []ImageData  `json:"images" dynamodb:"images"`
	ExtractedText string       `json:"extracted_text" dynamodb:"extracted_text"`
	Observation   string       `json:"observation" dynamodb:"observation"`
	User          string       `json:"user" dynamodb:"user"`
	ProcessedAt   time.Time    `json:"processed_at" dynamodb:"processed_at"`
	CreatedAt     time.Time    `json:"created_at" dynamodb:"created_at"`
	Status        string       `json:"status" dynamodb:"status"`
	Error         string       `json:"error,omitempty" dynamodb:"error,omitempty"`
	RetryCount    int          `json:"retry_count" dynamodb:"retry_count"`
	LastRetryAt   *time.Time   `json:"last_retry_at,omitempty" dynamodb:"last_retry_at,omitempty"`
}

// PersonalData contains personal information extracted from business card
type PersonalData struct {
	FullName   string `json:"full_name" dynamodb:"full_name"`
	FirstName  string `json:"first_name" dynamodb:"first_name"`
	LastName   string `json:"last_name" dynamodb:"last_name"`
	JobTitle   string `json:"job_title" dynamodb:"job_title"`
	Department string `json:"department" dynamodb:"department"`
	Email      string `json:"email" dynamodb:"email"`
	Phone      string `json:"phone" dynamodb:"phone"`
	Mobile     string `json:"mobile" dynamodb:"mobile"`
	LinkedIn   string `json:"linkedin" dynamodb:"linkedin"`
	Website    string `json:"website" dynamodb:"website"`
}

// CompanyData contains company information extracted from business card
type CompanyData struct {
	Name        string  `json:"name" dynamodb:"name"`
	Industry    string  `json:"industry" dynamodb:"industry"`
	Website     string  `json:"website" dynamodb:"website"`
	Email       string  `json:"email" dynamodb:"email"`
	Phone       string  `json:"phone" dynamodb:"phone"`
	Address     Address `json:"address" dynamodb:"address"`
	SocialMedia struct {
		LinkedIn  string `json:"linkedin" dynamodb:"linkedin"`
		Twitter   string `json:"twitter" dynamodb:"twitter"`
		Facebook  string `json:"facebook" dynamodb:"facebook"`
		Instagram string `json:"instagram" dynamodb:"instagram"`
	} `json:"social_media" dynamodb:"social_media"`
}

// Address represents the company address
type Address struct {
	Street     string `json:"street" dynamodb:"street"`
	City       string `json:"city" dynamodb:"city"`
	State      string `json:"state" dynamodb:"state"`
	PostalCode string `json:"postal_code" dynamodb:"postal_code"`
	Country    string `json:"country" dynamodb:"country"`
	Full       string `json:"full" dynamodb:"full"`
}

// ImageData represents uploaded image information
type ImageData struct {
	FileName    string    `json:"file_name" dynamodb:"file_name"`
	ContentType string    `json:"content_type" dynamodb:"content_type"`
	Size        int64     `json:"size" dynamodb:"size"`
	S3Key       string    `json:"s3_key" dynamodb:"s3_key"`
	S3URL       string    `json:"s3_url" dynamodb:"s3_url"`
	Data        []byte    `json:"data,omitempty" dynamodb:"-"`
	Base64Data  string    `json:"base64_data" dynamodb:"-"`
	UploadedAt  time.Time `json:"uploaded_at" dynamodb:"uploaded_at"`
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

// Base64ImageUpload represents an image uploaded as base64
type Base64ImageUpload struct {
	FileName     string `json:"file_name"`
	ContentType  string `json:"content_type"`
	Size         int64  `json:"size"`
	Base64Data   string `json:"base64_data"`
	LastModified int64  `json:"last_modified"`
}

// Base64BusinessCardRequest represents the request payload for processing business cards with base64 images
type Base64BusinessCardRequest struct {
	Images      []Base64ImageUpload `json:"images"`
	Timestamp   string              `json:"timestamp"`
	TotalImages int                 `json:"total_images"`
	Observation string              `json:"observation"`
	User        string              `json:"user"`
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

// UpdateObservationRequest represents the request for updating observation
type UpdateObservationRequest struct {
	Observation string `json:"observation"`
}

// BusinessCardStatus represents the possible states of a business card
const (
	StatusPending    = "PENDING"
	StatusProcessing = "PROCESSING"
	StatusCompleted  = "COMPLETED"
	StatusFailed     = "FAILED"
	StatusRetrying   = "RETRYING"
)
