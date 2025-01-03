package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"promocode-lambda/models"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// validateEpochExpiration checks if the expiration epoch timestamp is in the future
func validateEpochExpiration(epoch int64) error {
	log.Printf("[DEBUG] Validating epoch timestamp: %d", epoch)

	// Convert epoch to time.Time
	expirationTime := time.Unix(epoch, 0)
	currentTime := time.Now()

	log.Printf("[DEBUG] Expiration time: %v, Current time: %v", expirationTime, currentTime)

	// Compare with current time
	if expirationTime.Before(currentTime) {
		log.Printf("[ERROR] Expiration date is in the past. Expiration: %v, Current: %v", expirationTime, currentTime)
		return fmt.Errorf("expiration date must be in the future, got epoch: %d which resolves to %v", epoch, expirationTime)
	}

	log.Printf("[DEBUG] Epoch validation successful")
	return nil
}

// HandleCampaign processes campaign JSON files from S3 and stores them in DynamoDB
func HandleCampaign(ctx context.Context, event events.S3EventRecord, data []byte, dynamoClient DynamoDBAPI) error {
	log.Printf("[INFO] Starting campaign processing for file: %s", event.S3.Object.Key)
	log.Printf("[DEBUG] Received data length: %d bytes", len(data))

	// Validate input data
	if len(data) == 0 {
		log.Printf("[ERROR] Empty campaign data received")
		return fmt.Errorf("empty campaign data received")
	}

	// Parse campaign JSON
	var campaign models.Campaign
	if err := json.Unmarshal(data, &campaign); err != nil {
		log.Printf("[ERROR] Failed to decode campaign JSON: %v", err)
		log.Printf("[DEBUG] Raw data: %s", string(data))
		return fmt.Errorf("failed to decode campaign JSON: %v", err)
	}

	log.Printf("[DEBUG] Parsed campaign data: %+v", campaign)

	// Validate required fields
	if campaign.CampaignID == "" {
		log.Printf("[ERROR] Missing required field: campaignId")
		return fmt.Errorf("campaignId is required")
	}

	// Validate expiration epoch timestamp
	if err := validateEpochExpiration(campaign.Expiration); err != nil {
		log.Printf("[ERROR] Expiration validation failed for campaign %s: %v", campaign.CampaignID, err)
		return fmt.Errorf("expiration validation failed: %v", err)
	}

	log.Printf("[DEBUG] Creating DynamoDB put request for campaign: %s", campaign.CampaignID)

	// Create a new campaign object in DynamoDB(Put Method)
	_, err := dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String("ads-qrcode-promotions-qe-ft-campaign"),
		Item: map[string]types.AttributeValue{
			"campaignId":    &types.AttributeValueMemberS{Value: campaign.CampaignID},
			"advertiser":    &types.AttributeValueMemberS{Value: campaign.Advertiser},
			"baseUrl":       &types.AttributeValueMemberS{Value: campaign.BaseURL},
			"active":        &types.AttributeValueMemberBOOL{Value: campaign.Active},
			"expiration":    &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", campaign.Expiration)},
			"campaignType":  &types.AttributeValueMemberS{Value: campaign.CampaignType},
			"platformType": &types.AttributeValueMemberS{Value: campaign.PlatformType},
		},
		ConditionExpression: aws.String("attribute_not_exists(campaignId)"),
	})

	if err != nil {
		// If campaign exists, update it
		if strings.Contains(err.Error(), "ConditionalCheckFailedException") {
			log.Printf("[INFO] Campaign %s already exists, attempting update", campaign.CampaignID)
			return updateCampaign(ctx, dynamoClient, campaign)
		}
		log.Printf("[ERROR] Failed to create campaign in DynamoDB: %v", err)
		return fmt.Errorf("failed to create campaign in DynamoDB: %v", err)
	}

	log.Printf("[INFO] Campaign %s successfully created in DynamoDB", campaign.CampaignID)
	return nil
}

// updateCampaign updates an existing campaign in DynamoDB(Post Method)
func updateCampaign(ctx context.Context, dynamoClient DynamoDBAPI, campaign models.Campaign) error {
	log.Printf("[INFO] Starting update for campaign: %s", campaign.CampaignID)
	log.Printf("[DEBUG] Update payload: %+v", campaign)

	// Validate expiration epoch timestamp before update
	if err := validateEpochExpiration(campaign.Expiration); err != nil {
		log.Printf("[ERROR] Expiration validation failed for update of campaign %s: %v", campaign.CampaignID, err)
		return fmt.Errorf("expiration validation failed: %v", err)
	}

	updateInput := &dynamodb.UpdateItemInput{
		TableName: aws.String("ads-qrcode-promotions-qe-ft-campaign"),
		Key: map[string]types.AttributeValue{
			"campaignId": &types.AttributeValueMemberS{Value: campaign.CampaignID},
		},
		UpdateExpression: aws.String("SET advertiser = :a, baseUrl = :b, active = :act, expiration = :exp, campaignType = :ct, platformType = :pt"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":a":   &types.AttributeValueMemberS{Value: campaign.Advertiser},
			":b":   &types.AttributeValueMemberS{Value: campaign.BaseURL},
			":act": &types.AttributeValueMemberBOOL{Value: campaign.Active},
			":exp": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", campaign.Expiration)},
			":ct":  &types.AttributeValueMemberS{Value: campaign.CampaignType},
			":pt":  &types.AttributeValueMemberS{Value: campaign.PlatformType},
		},
	}

	log.Printf("[DEBUG] DynamoDB update input: %+v", updateInput)

	_, err := dynamoClient.UpdateItem(ctx, updateInput)
	if err != nil {
		log.Printf("[ERROR] Failed to update campaign %s in DynamoDB: %v", campaign.CampaignID, err)
		return fmt.Errorf("failed to update campaign in DynamoDB: %v", err)
	}

	log.Printf("[INFO] Campaign %s successfully updated in DynamoDB", campaign.CampaignID)
	return nil
}