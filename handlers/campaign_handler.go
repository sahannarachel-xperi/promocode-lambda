package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"promocode-lambda/models"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// HandleCampaign processes campaign JSON files from S3 and stores them in DynamoDB
func HandleCampaign(ctx context.Context, event events.S3EventRecord, data []byte, dynamoClient DynamoDBAPI) error {
	log.Printf("Processing file: %s", event.S3.Object.Key)

	// Validate input data
	if len(data) == 0 {
		return fmt.Errorf("empty campaign data received")
	}

	// Parse campaign JSON
	var campaign models.Campaign
	if err := json.Unmarshal(data, &campaign); err != nil {
		return fmt.Errorf("failed to decode campaign JSON: %v", err)
	}

	// Validate required fields
	if campaign.CampaignID == "" {
		return fmt.Errorf("campaignId is required")
	}

	// Create a new campaign object in DynamoDB(Put Method)
	_, err := dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String("ads-qrcode-promotions-qe-ft-campaign"),
		Item: map[string]types.AttributeValue{
			"campaignId":   &types.AttributeValueMemberS{Value: campaign.CampaignID},
			"advertiser":   &types.AttributeValueMemberS{Value: campaign.Advertiser},
			"baseUrl":      &types.AttributeValueMemberS{Value: campaign.BaseURL},
			"enabled":      &types.AttributeValueMemberBOOL{Value: campaign.Enabled},
			"expiration":   &types.AttributeValueMemberS{Value: campaign.Expiration},
			"campaignType": &types.AttributeValueMemberS{Value: campaign.CampaignType},
		},
		ConditionExpression: aws.String("attribute_not_exists(campaignId)"),
	})
	if err != nil {
		// If campaign exists, update it
		if strings.Contains(err.Error(), "ConditionalCheckFailedException") {
			return updateCampaign(ctx, dynamoClient, campaign)
		}
		return fmt.Errorf("failed to create campaign in DynamoDB: %v", err)
	}

	log.Printf("Campaign %s successfully created in DynamoDB", campaign.CampaignID)
	return nil
}

// updateCampaign updates an existing campaign in DynamoDB(Post Method)
func updateCampaign(ctx context.Context, dynamoClient DynamoDBAPI, campaign models.Campaign) error {
	_, err := dynamoClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String("ads-qrcode-promotions-qe-ft-campaign"),
		Key: map[string]types.AttributeValue{
			"campaignId": &types.AttributeValueMemberS{Value: campaign.CampaignID},
		},
		UpdateExpression: aws.String("SET advertiser = :a, baseUrl = :b, enabled = :e, expiration = :exp, campaignType = :ct"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":a":   &types.AttributeValueMemberS{Value: campaign.Advertiser},
			":b":   &types.AttributeValueMemberS{Value: campaign.BaseURL},
			":e":   &types.AttributeValueMemberBOOL{Value: campaign.Enabled},
			":exp": &types.AttributeValueMemberS{Value: campaign.Expiration},
			":ct":  &types.AttributeValueMemberS{Value: campaign.CampaignType},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to update campaign in DynamoDB: %v", err)
	}
	log.Printf("Campaign %s successfully updated in DynamoDB", campaign.CampaignID)
	return nil
}
