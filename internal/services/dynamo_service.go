package services

import (
	"context"
	"fmt"
	"log"

	"business-card-reader/internal/models"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type DynamoService struct {
	client    *dynamodb.Client
	tableName string
}

func NewDynamoService(region string) (*DynamoService, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		log.Printf("[DynamoService] Failed to load AWS config: %v", err)
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := dynamodb.NewFromConfig(cfg)
	tableName := "business-card-reader" // Updated table name

	log.Printf("[DynamoService] Initialized with table: %s, region: %s", tableName, region)

	return &DynamoService{
		client:    client,
		tableName: tableName,
	}, nil
}

func (d *DynamoService) SaveBusinessCard(ctx context.Context, businessCard *models.BusinessCard) error {
	log.Printf("[DynamoService] Saving business card with ID: %s", businessCard.ID)
	item, err := attributevalue.MarshalMap(businessCard)
	if err != nil {
		log.Printf("[DynamoService] Failed to marshal business card: %v", err)
		return fmt.Errorf("failed to marshal business card: %w", err)
	}

	_, err = d.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(d.tableName),
		Item:      item,
	})
	if err != nil {
		log.Printf("[DynamoService] Failed to save business card to DynamoDB: %v", err)
		return fmt.Errorf("failed to save business card: %w", err)
	}

	log.Printf("[DynamoService] Successfully saved business card with ID: %s", businessCard.ID)
	return nil
}

func (d *DynamoService) GetBusinessCard(ctx context.Context, id string) (*models.BusinessCard, error) {
	result, err := d.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(d.tableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: id},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get business card: %w", err)
	}

	if result.Item == nil {
		return nil, fmt.Errorf("business card not found")
	}

	var businessCard models.BusinessCard
	err = attributevalue.UnmarshalMap(result.Item, &businessCard)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal business card: %w", err)
	}

	return &businessCard, nil
}

func (d *DynamoService) GetAllBusinessCards(ctx context.Context) ([]models.BusinessCard, error) {
	result, err := d.client.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(d.tableName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to scan business cards: %w", err)
	}

	var businessCards []models.BusinessCard
	for _, item := range result.Items {
		var businessCard models.BusinessCard
		err = attributevalue.UnmarshalMap(item, &businessCard)
		if err != nil {
			continue // Skip items that can't be unmarshaled
		}
		businessCards = append(businessCards, businessCard)
	}

	return businessCards, nil
}

func (d *DynamoService) CreateTableIfNotExists(ctx context.Context) error {
	// Check if table exists
	_, err := d.client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(d.tableName),
	})
	if err == nil {
		// Table already exists
		return nil
	}

	// Create table
	_, err = d.client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(d.tableName),
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("id"),
				KeyType:       types.KeyTypeHash,
			},
		},
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("id"),
				AttributeType: types.ScalarAttributeTypeS, // String type for GUID
			},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	return nil
}

func (d *DynamoService) GetBusinessCardsByStatus(ctx context.Context, status string) ([]models.BusinessCard, error) {
	// Use a filter expression to get cards by status
	result, err := d.client.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String(d.tableName),
		FilterExpression: aws.String("#status = :status"),
		ExpressionAttributeNames: map[string]string{
			"#status": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":status": &types.AttributeValueMemberS{Value: status},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to scan business cards by status: %w", err)
	}

	var businessCards []models.BusinessCard
	for _, item := range result.Items {
		var businessCard models.BusinessCard
		err = attributevalue.UnmarshalMap(item, &businessCard)
		if err != nil {
			continue // Skip items that can't be unmarshaled
		}
		businessCards = append(businessCards, businessCard)
	}

	return businessCards, nil
}
