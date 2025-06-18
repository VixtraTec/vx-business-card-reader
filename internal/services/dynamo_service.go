package services

import (
	"context"
	"fmt"

	"business-card-reader/internal/logger"
	"business-card-reader/internal/models"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

type DynamoService struct {
	client    *dynamodb.DynamoDB
	tableName string
}

func NewDynamoService(region string) (*DynamoService, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		logger.LogError("DynamoService", err, map[string]interface{}{
			"step":   "create_session",
			"region": region,
		})
		return nil, fmt.Errorf("failed to create AWS session: %w", err)
	}

	client := dynamodb.New(sess)
	tableName := "business-card-reader"

	logger.LogInfo("DynamoService", "Initialized DynamoDB service", map[string]interface{}{
		"table_name":  tableName,
		"region":      region,
		"sdk_version": "v1",
	})

	return &DynamoService{
		client:    client,
		tableName: tableName,
	}, nil
}

func (d *DynamoService) SaveBusinessCard(ctx context.Context, businessCard *models.BusinessCard) error {
	logger.LogInfo("DynamoSaveBusinessCard", "Starting save to DynamoDB", map[string]interface{}{
		"business_card_id": businessCard.ID,
		"status":           businessCard.Status,
		"table_name":       d.tableName,
		"sdk_version":      "v1",
	})

	item, err := dynamodbattribute.MarshalMap(businessCard)
	if err != nil {
		logger.LogError("DynamoSaveBusinessCard", err, map[string]interface{}{
			"step":             "marshal_business_card",
			"business_card_id": businessCard.ID,
		})
		return fmt.Errorf("failed to marshal business card: %w", err)
	}

	logger.LogDebug("DynamoSaveBusinessCard", "Business card marshaled successfully", map[string]interface{}{
		"business_card_id": businessCard.ID,
		"item_keys":        len(item),
	})

	_, err = d.client.PutItemWithContext(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(d.tableName),
		Item:      item,
	})
	if err != nil {
		logger.LogError("DynamoSaveBusinessCard", err, map[string]interface{}{
			"step":             "put_item",
			"business_card_id": businessCard.ID,
			"table_name":       d.tableName,
			"sdk_version":      "v1",
		})
		return fmt.Errorf("failed to save business card: %w", err)
	}

	logger.LogInfo("DynamoSaveBusinessCard", "Business card saved successfully", map[string]interface{}{
		"business_card_id": businessCard.ID,
	})
	return nil
}

func (d *DynamoService) GetBusinessCard(ctx context.Context, id string) (*models.BusinessCard, error) {
	logger.LogInfo("DynamoGetBusinessCard", "Getting business card from DynamoDB", map[string]interface{}{
		"business_card_id": id,
		"table_name":       d.tableName,
		"sdk_version":      "v1",
	})

	result, err := d.client.GetItemWithContext(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(d.tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"id": {
				S: aws.String(id),
			},
		},
	})
	if err != nil {
		logger.LogError("DynamoGetBusinessCard", err, map[string]interface{}{
			"business_card_id": id,
			"table_name":       d.tableName,
		})
		return nil, fmt.Errorf("failed to get business card: %w", err)
	}

	if result.Item == nil {
		logger.LogWarn("DynamoGetBusinessCard", "Business card not found", map[string]interface{}{
			"business_card_id": id,
		})
		return nil, fmt.Errorf("business card not found")
	}

	var businessCard models.BusinessCard
	err = dynamodbattribute.UnmarshalMap(result.Item, &businessCard)
	if err != nil {
		logger.LogError("DynamoGetBusinessCard", err, map[string]interface{}{
			"business_card_id": id,
			"step":             "unmarshal",
		})
		return nil, fmt.Errorf("failed to unmarshal business card: %w", err)
	}

	logger.LogInfo("DynamoGetBusinessCard", "Business card retrieved successfully", map[string]interface{}{
		"business_card_id": id,
	})

	return &businessCard, nil
}

func (d *DynamoService) GetAllBusinessCards(ctx context.Context) ([]models.BusinessCard, error) {
	logger.LogInfo("DynamoGetAllBusinessCards", "Scanning all business cards", map[string]interface{}{
		"table_name":  d.tableName,
		"sdk_version": "v1",
	})

	result, err := d.client.ScanWithContext(ctx, &dynamodb.ScanInput{
		TableName: aws.String(d.tableName),
	})
	if err != nil {
		logger.LogError("DynamoGetAllBusinessCards", err, map[string]interface{}{
			"table_name": d.tableName,
		})
		return nil, fmt.Errorf("failed to scan business cards: %w", err)
	}

	var businessCards []models.BusinessCard
	for _, item := range result.Items {
		var businessCard models.BusinessCard
		err = dynamodbattribute.UnmarshalMap(item, &businessCard)
		if err != nil {
			logger.LogWarn("DynamoGetAllBusinessCards", "Failed to unmarshal item, skipping", map[string]interface{}{
				"error": err.Error(),
			})
			continue // Skip items that can't be unmarshaled
		}
		businessCards = append(businessCards, businessCard)
	}

	logger.LogInfo("DynamoGetAllBusinessCards", "Scan completed", map[string]interface{}{
		"count": len(businessCards),
	})

	return businessCards, nil
}

func (d *DynamoService) CreateTableIfNotExists(ctx context.Context) error {
	logger.LogInfo("DynamoCreateTable", "Checking if table exists", map[string]interface{}{
		"table_name":  d.tableName,
		"sdk_version": "v1",
	})

	// Check if table exists
	_, err := d.client.DescribeTableWithContext(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(d.tableName),
	})
	if err == nil {
		logger.LogInfo("DynamoCreateTable", "Table already exists", map[string]interface{}{
			"table_name": d.tableName,
		})
		return nil
	}

	logger.LogInfo("DynamoCreateTable", "Creating new table", map[string]interface{}{
		"table_name": d.tableName,
	})

	// Create table
	_, err = d.client.CreateTableWithContext(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(d.tableName),
		KeySchema: []*dynamodb.KeySchemaElement{
			{
				AttributeName: aws.String("id"),
				KeyType:       aws.String("HASH"),
			},
		},
		AttributeDefinitions: []*dynamodb.AttributeDefinition{
			{
				AttributeName: aws.String("id"),
				AttributeType: aws.String("S"), // String type for GUID
			},
		},
		BillingMode: aws.String("PAY_PER_REQUEST"),
	})
	if err != nil {
		logger.LogError("DynamoCreateTable", err, map[string]interface{}{
			"table_name": d.tableName,
		})
		return fmt.Errorf("failed to create table: %w", err)
	}

	logger.LogInfo("DynamoCreateTable", "Table created successfully", map[string]interface{}{
		"table_name": d.tableName,
	})

	return nil
}

func (d *DynamoService) GetBusinessCardsByStatus(ctx context.Context, status string) ([]models.BusinessCard, error) {
	logger.LogInfo("DynamoGetByStatus", "Scanning business cards by status", map[string]interface{}{
		"status":      status,
		"table_name":  d.tableName,
		"sdk_version": "v1",
	})

	// Use a filter expression to get cards by status
	result, err := d.client.ScanWithContext(ctx, &dynamodb.ScanInput{
		TableName:        aws.String(d.tableName),
		FilterExpression: aws.String("#status = :status"),
		ExpressionAttributeNames: map[string]*string{
			"#status": aws.String("status"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":status": {
				S: aws.String(status),
			},
		},
	})
	if err != nil {
		logger.LogError("DynamoGetByStatus", err, map[string]interface{}{
			"status":     status,
			"table_name": d.tableName,
		})
		return nil, fmt.Errorf("failed to scan business cards by status: %w", err)
	}

	var businessCards []models.BusinessCard
	for _, item := range result.Items {
		var businessCard models.BusinessCard
		err = dynamodbattribute.UnmarshalMap(item, &businessCard)
		if err != nil {
			logger.LogWarn("DynamoGetByStatus", "Failed to unmarshal item, skipping", map[string]interface{}{
				"error": err.Error(),
			})
			continue // Skip items that can't be unmarshaled
		}
		businessCards = append(businessCards, businessCard)
	}

	logger.LogInfo("DynamoGetByStatus", "Status scan completed", map[string]interface{}{
		"status": status,
		"count":  len(businessCards),
	})

	return businessCards, nil
}
