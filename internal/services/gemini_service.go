package services

import (
	"context"
	"encoding/json"
	"fmt"

	"business-card-reader/internal/models"

	"google.golang.org/genai"
)

type GeminiService struct {
	client    *genai.Client
	modelName string
}

func NewGeminiService(apiKey string, modelName string) (*GeminiService, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	return &GeminiService{
		client:    client,
		modelName: modelName,
	}, nil
}

func (g *GeminiService) ExtractBusinessCardData(ctx context.Context, images []models.ImageData) (*models.BusinessCard, error) {
	prompt := g.buildExtractionPrompt()

	// Prepare parts for the request
	parts := []*genai.Part{{Text: prompt}}

	// Add images to the request
	for _, img := range images {
		parts = append(parts, &genai.Part{
			InlineData: &genai.Blob{Data: img.Data, MIMEType: img.ContentType},
		})
	}

	contents := []*genai.Content{{Parts: parts}}
	resp, err := g.client.Models.GenerateContent(ctx, g.modelName, contents, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no content generated")
	}

	// Extract the JSON response
	responseText := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0].Text)

	// Clean the response to extract JSON
	jsonStr := g.extractJSONFromResponse(responseText)

	// Parse the extracted data
	var extractedData struct {
		PersonalData models.PersonalData `json:"personal_data"`
		CompanyData  models.CompanyData  `json:"company_data"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &extractedData); err != nil {
		return nil, fmt.Errorf("failed to parse extracted data: %w", err)
	}

	businessCard := &models.BusinessCard{
		PersonalData:  extractedData.PersonalData,
		CompanyData:   extractedData.CompanyData,
		ExtractedText: responseText,
		Images:        images,
	}

	return businessCard, nil
}

func (g *GeminiService) buildExtractionPrompt() string {
	return `
You are an expert at extracting information from business cards. Analyze the provided business card image(s) and extract all relevant information.

Please extract the information and return it in the following JSON format:

{
  "personal_data": {
    "full_name": "",
    "first_name": "",
    "last_name": "",
    "job_title": "",
    "department": "",
    "email": "",
    "phone": "",
    "mobile": "",
    "linkedin": "",
    "website": ""
  },
  "company_data": {
    "name": "",
    "industry": "",
    "website": "",
    "email": "",
    "phone": "",
    "address": {
      "street": "",
      "city": "",
      "state": "",
      "postal_code": "",
      "country": "",
      "full": ""
    },
    "social_media": {
      "linkedin": "",
      "twitter": "",
      "facebook": "",
      "instagram": ""
    }
  }
}

Rules:
1. Extract all visible text accurately
2. If multiple images are provided, combine information from both
3. Leave fields empty ("") if information is not available
4. For phone numbers, distinguish between main phone and mobile if possible
5. For websites, include the full URL if visible
6. For social media, extract usernames or full URLs
7. For addresses, provide both individual components and full address
8. Return ONLY the JSON object, no additional text or formatting

Analyze the business card(s) and extract the information:
`
}

func (g *GeminiService) extractJSONFromResponse(response string) string {
	// Find the JSON object in the response
	start := -1
	end := -1
	braceCount := 0

	for i, char := range response {
		if char == '{' {
			if start == -1 {
				start = i
			}
			braceCount++
		} else if char == '}' {
			braceCount--
			if braceCount == 0 && start != -1 {
				end = i + 1
				break
			}
		}
	}

	if start != -1 && end != -1 {
		return response[start:end]
	}

	return response
}
