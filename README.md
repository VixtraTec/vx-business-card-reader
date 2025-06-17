# Business Card Reader API

A Go-based REST API that processes business card images using Google's Gemini AI to extract structured data and stores it in DynamoDB with images stored in AWS S3.

## Features

- **Image Upload**: Accept up to 2 business card images per request
- **AI Processing**: Uses Google Gemini AI to extract structured data from business cards
- **S3 Storage**: Images are stored in AWS S3 for efficient and scalable storage
- **Data Storage**: Stores extracted data and S3 references in AWS DynamoDB
- **Structured Output**: Consistent JSON format for all extracted data
- **REST API**: Clean endpoints for processing and retrieving business card data
- **Retry Failed Processing**: Ability to retry processing for failed business cards
- **Swagger Documentation**: Comprehensive API documentation
- **Comprehensive Logging**: Detailed logging system
- **Secure Access**: Images accessed via presigned URLs for security

## Project Structure

```
business-card-reader/
├── main.go                          # Application entry point
├── go.mod                           # Go module dependencies
├── internal/
│   ├── config/
│   │   └── config.go               # Configuration management
│   ├── models/
│   │   └── business_card.go        # Data models and structures
│   ├── services/
│   │   ├── business_card_service.go # Main business logic
│   │   ├── dynamo_service.go       # DynamoDB operations
│   │   ├── gemini_service.go       # Gemini AI integration
│   │   └── s3_service.go           # S3 storage operations
│   ├── handlers/
│   │   └── business_card_handler.go # HTTP request handlers
│   └── logger/
│       └── logger.go               # Logging configuration
├── .env.example                     # Environment variables template
└── README.md                       # This file
```

## Prerequisites

- Go 1.21 or higher
- AWS Account with DynamoDB and S3 access
- Google Cloud Account with Gemini API access

## Setup

### 1. Clone the repository
```bash
git clone <repository-url>
cd business-card-reader
```

### 2. Install dependencies
```bash
go mod tidy
```

### 3. Set up environment variables
```bash
cp .env.example .env
```

Edit `.env` file with your actual values:
```env
GEMINI_API_KEY=your_gemini_api_key_here
GEMINI_MODEL_NAME=gemini-1.5-flash
AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=your_aws_access_key_id
AWS_SECRET_ACCESS_KEY=your_aws_secret_access_key
S3_BUCKET_NAME=vx-src-api-test
DYNAMODB_TABLE_NAME=business-cards
PORT=8080
LOG_LEVEL=info
```

### 4. Get API Keys

#### Google Gemini API Key:
1. Go to [Google AI Studio](https://makersuite.google.com/app/apikey)
2. Create a new API key
3. Copy the API key to your `.env` file

#### AWS Credentials:
1. Log in to AWS Console
2. Go to IAM → Users → Create user
3. Attach policies: `AmazonDynamoDBFullAccess`, `AmazonS3FullAccess`
4. Create access key and add to `.env` file

### 5. Create AWS Resources

#### DynamoDB Table:
The application will automatically create the table if it doesn't exist, or you can create it manually:

```bash
aws dynamodb create-table \
    --table-name business-cards \
    --attribute-definitions AttributeName=id,AttributeType=S \
    --key-schema AttributeName=id,KeyType=HASH \
    --billing-mode PAY_PER_REQUEST \
    --region us-east-1
```

#### S3 Bucket:
Create the S3 bucket for image storage:

```bash
aws s3 mb s3://vx-src-api-test --region us-east-1
```

## Running the Application

### Development
```bash
go run main.go
```

### Production
```bash
go build -o business-card-reader main.go
./business-card-reader
```

The server will start on port 8080 (or the port specified in your `.env` file).

## API Endpoints

### 1. Process Business Card
**POST** `/api/v1/business-cards`

Upload and process business card images.

**Request:**
- Content-Type: `multipart/form-data`
- Form field: `images` (1-2 image files)
- Supported formats: JPEG, PNG, WebP

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "uuid-string",
    "personal_data": {
      "full_name": "John Doe",
      "first_name": "John",
      "last_name": "Doe",
      "job_title": "Software Engineer",
      "department": "Engineering",
      "email": "john.doe@example.com",
      "phone": "+1-555-0123",
      "mobile": "+1-555-0124",
      "linkedin": "linkedin.com/in/johndoe",
      "website": "johndoe.dev"
    },
    "company_data": {
      "name": "Tech Corp",
      "industry": "Technology",
      "website": "techcorp.com",
      "email": "info@techcorp.com",
      "phone": "+1-555-0100",
      "address": {
        "street": "123 Tech Street",
        "city": "San Francisco",
        "state": "CA",
        "postal_code": "94105",
        "country": "USA",
        "full": "123 Tech Street, San Francisco, CA 94105, USA"
      },
      "social_media": {
        "linkedin": "linkedin.com/company/techcorp",
        "twitter": "@techcorp",
        "facebook": "facebook.com/techcorp",
        "instagram": "@techcorp"
      }
    },
    "processed_at": "2024-01-15T10:30:00Z",
    "created_at": "2024-01-15T10:30:00Z"
  }
}
```

### 2. Get All Business Cards
**GET** `/api/v1/business-cards`

Retrieve all processed business cards with presigned URLs for images.

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "id": "uuid-string",
      "personal_data": {...},
      "company_data": {...},
      "images": [
        {
          "file_name": "business_card.jpg",
          "content_type": "image/jpeg",
          "size": 1024576,
          "s3_key": "business-cards/uuid-string/image-uuid.jpg",
          "s3_url": "https://vx-src-api-test.s3.amazonaws.com/...",
          "uploaded_at": "2024-01-15T10:30:00Z"
        }
      ],
      "processed_at": "2024-01-15T10:30:00Z",
      "created_at": "2024-01-15T10:30:00Z"
    }
  ],
  "count": 10
}
```

### 3. Get Business Card by ID
**GET** `/api/v1/business-cards/{id}`

Retrieve a specific business card by ID with presigned URLs for images.

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "uuid-string",
    "personal_data": {...},
    "company_data": {...},
    "images": [
      {
        "file_name": "business_card.jpg",
        "content_type": "image/jpeg",
        "size": 1024576,
        "s3_key": "business-cards/uuid-string/image-uuid.jpg",
        "s3_url": "https://vx-src-api-test.s3.amazonaws.com/...",
        "uploaded_at": "2024-01-15T10:30:00Z"
      }
    ],
    "processed_at": "2024-01-15T10:30:00Z",
    "created_at": "2024-01-15T10:30:00Z"
  }
}
```

### 4. Delete Business Card
**DELETE** `/api/v1/business-cards/{id}`

Delete a business card and its associated images from both DynamoDB and S3.

**Response:**
```json
{
  "success": true,
  "message": "Business card deleted successfully"
}
```

### 5. Health Check
**GET** `/health`

Check if the service is running.

**Response:**
```json
{
  "status": "healthy"
}
```

### 6. Retry Failed Processing
**POST** `/api/v1/business-cards/{id}/retry`

Retry processing for a failed business card.

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "uuid-string",
    "personal_data": {...},
    "company_data": {...},
    "processed_at": "2024-01-15T10:30:00Z",
    "created_at": "2024-01-15T10:30:00Z"
  }
}
```

### 7. Get Failed Business Cards
**GET** `/api/v1/business-cards/failed`

Retrieve all failed business cards with presigned URLs for images.

**Response:**
```json
{
  "success": true,
  "data": [...],
  "count": 10
}
```

### 8. API Documentation
**GET** `/swagger/`

Retrieve Swagger documentation for the API.

## Example Usage

### Using cURL
```bash
# Process a business card
curl -X POST http://localhost:8080/api/v1/business-cards \
  -F "images=@business_card_1.jpg" \
  -F "images=@business_card_2.jpg"

# Get all business cards
curl http://localhost:8080/api/v1/business-cards

# Get specific business card
curl http://localhost:8080/api/v1/business-cards/{id}

# Delete a business card
curl -X DELETE http://localhost:8080/api/v1/business-cards/{id}
```

### Using JavaScript/Fetch
```javascript
const formData = new FormData();
formData.append('images', file1);
formData.append('images', file2);

const response = await fetch('/api/v1/business-cards', {
  method: 'POST',
  body: formData
});

const result = await response.json();
```

## Data Structure

The API returns consistent JSON structure for all business cards:

- **Personal Data**: Name, contact info, job title, personal links
- **Company Data**: Company name, address, contact info, social media
- **Images**: S3 metadata with presigned URLs for secure access
- **Metadata**: Processing timestamps, unique ID

### Image Storage Structure

Images are stored in S3 with the following structure:
```
s3://vx-src-api-test/
└── business-cards/
    └── {business_card_id}/
        ├── {uuid1}.jpg
        ├── {uuid2}.png
        └── ...
```

### Image Access

- Images are accessed via presigned URLs that expire after 1 hour
- URLs are generated dynamically when retrieving business card data
- No direct public access to S3 objects for security

## Error Handling

All endpoints return consistent error responses:

```json
{
  "success": false,
  "error": "Error description"
}
```

Common HTTP status codes:
- `200`: Success
- `400`: Bad Request (invalid input)
- `404`: Not Found
- `500`: Internal Server Error

## Development

### Running Tests
```bash
go test ./...
```

### Code Structure
- `internal/models/`: Data structures and types
- `internal/services/`: Business logic and external service integrations
- `internal/handlers/`: HTTP request handling
- `internal/config/`: Configuration management
- `internal/logger/`: Logging configuration

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `GEMINI_API_KEY` | Google Gemini AI API key | Required |
| `GEMINI_MODEL_NAME` | Gemini model to use | `gemini-1.5-flash` |
| `AWS_REGION` | AWS region for DynamoDB and S3 | `us-east-1` |
| `AWS_ACCESS_KEY_ID` | AWS access key | Required |
| `AWS_SECRET_ACCESS_KEY` | AWS secret key | Required |
| `S3_BUCKET_NAME` | S3 bucket name for image storage | `vx-src-api-test` |
| `DYNAMODB_TABLE_NAME` | DynamoDB table name | `business-cards` |
| `PORT` | Server port | `8080` |
| `GIN_MODE` | Gin framework mode | `debug` |
| `LOG_LEVEL` | Logging level | `info` |

## Architecture

The application follows a clean architecture pattern:

1. **Image Upload**: Images are uploaded via multipart form
2. **S3 Storage**: Images are immediately stored in S3 with unique keys
3. **AI Processing**: Gemini AI processes images downloaded from S3
4. **Data Storage**: Extracted data and S3 references stored in DynamoDB
5. **Image Access**: Presigned URLs generated for secure image access

### Processing Flow

```
Upload → S3 Storage → AI Processing → DynamoDB Storage → Response
   ↓
Images stored with structure:
business-cards/{id}/{uuid}.{ext}
```

## Deployment

### Docker (Optional)
Create a `Dockerfile`:
```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o main .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/main .
CMD ["./main"]
```

### AWS Lambda (Optional)
The application can be deployed to AWS Lambda with minimal modifications using the AWS Lambda Go runtime.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

This project is licensed under the MIT License.

## Logging

The application includes a comprehensive logging system that tracks all operations:

### Log Levels

- **DEBUG**: Detailed information for debugging
- **INFO**: General information about application flow
- **WARN**: Warning messages for potential issues
- **ERROR**: Error messages for failed operations

### Logged Operations

1. **HTTP Requests**: All incoming requests and responses
2. **Business Card Processing**: Complete processing pipeline
3. **Database Operations**: DynamoDB read/write operations
4. **S3 Operations**: Image upload, download, and deletion operations
5. **AI Processing**: Gemini AI interactions
6. **Error Handling**: Detailed error information with context

### Log Format

Logs are output in JSON format with structured fields:

```json
{
  "level": "info",
  "time": "2024-01-01 12:00:00",
  "operation": "ProcessBusinessCard",
  "message": "Starting business card processing",
  "business_card_id": "uuid-here",
  "image_count": 2
}
```

### Viewing Logs

When running locally, logs are output to stdout. In production, you can:

1. Redirect logs to a file:
   ```bash
   go run main.go > app.log 2>&1
   ```

2. Use log aggregation services like CloudWatch, ELK Stack, or similar

3. Monitor specific operations by filtering JSON logs:
   ```bash
   go run main.go | jq 'select(.operation == "ProcessBusinessCard")'
   ```

This comprehensive logging helps with:
- Debugging production issues
- Monitoring application performance
- Tracking user behavior
- Identifying bottlenecks in the processing pipeline
- Monitoring S3 operations and costs 