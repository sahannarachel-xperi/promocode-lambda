package handlers

import (
	"context"
	"fmt"
	"log"
	"strings"

	"promocode-lambda/models"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// HandlePromoCode processes promo-code text files from S3 and stores them in DynamoDB.
func HandlePromoCode(ctx context.Context, event events.S3EventRecord, data []byte, dynamoClient DynamoDBAPI) error {
	log.Printf("Processing file: %s", event.S3.Object.Key)

	// Validate that we received non-empty data
	if len(data) == 0 {
		return fmt.Errorf("empty promocode data received")
	}

	// Ensure we're processing a text file
	if !strings.HasSuffix(event.S3.Object.Key, ".txt") {
		return fmt.Errorf("invalid file type, expected TXT file for promocodes, got: %s", event.S3.Object.Key)
	}

	// Parse and validate the S3 path structure
	// Expected format: qe-ft/promocodes/advertiser/campaign_id/uuid/filename.txt
	pathParts := strings.Split(event.S3.Object.Key, "/")
	if len(pathParts) < 6 {
		return fmt.Errorf("invalid path structure: %s, expected qe-ft/promocodes/advertiser/campaign_id/uuid/filename.txt", event.S3.Object.Key)
	}

	// Extract campaignId from the path (now at index 3)
	campaignId := pathParts[3]

	// Verify that the campaign exists in DynamoDB before processing promo codes
	_, err := dynamoClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String("ads-qrcode-promotions-qe-ft-campaign"),
		Key: map[string]types.AttributeValue{
			"campaignId": &types.AttributeValueMemberS{Value: campaignId},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to verify campaign exists: %v", err)
	}

	// Parse the promo codes from the text file
	content := string(data)
	lines := strings.Split(content, "\n")
	var promoCodes []models.PromoCode

	// Create PromoCode objects for each non-empty line
	for _, line := range lines {
		if line = strings.TrimSpace(line); line != "" {
			promoCodes = append(promoCodes, models.PromoCode{
				CampaignID:       campaignId,
				Code:             line,
				RedemptionStatus: false,
			})
		}
	}

	// Write all promo codes to DynamoDB using batch processing
	if err := batchWritePromocodes(ctx, dynamoClient, promoCodes); err != nil {
		return fmt.Errorf("failed to batch write promocodes: %v", err)
	}
	log.Printf("%d promocodes successfully processed", len(promoCodes))

	return nil
}


//batch processing
func batchWritePromocodes(ctx context.Context, dynamoClient DynamoDBAPI, codes []models.PromoCode) error {
	const batchSize = 25 // DynamoDB's maximum batch write size

	// Process promo codes in batches
	for i := 0; i < len(codes); i += batchSize {
		end := i + batchSize
		if end > len(codes) {
			end = len(codes)
		}

		batch := codes[i:end]
		writeRequests := make([]types.WriteRequest, len(batch))

		// Create write requests for each promocode in the batch
		for j, code := range batch {
			writeRequests[j] = types.WriteRequest{
				PutRequest: &types.PutRequest{
					Item: map[string]types.AttributeValue{
						"campaignId":       &types.AttributeValueMemberS{Value: code.CampaignID},
						"promocode":        &types.AttributeValueMemberS{Value: code.Code},
						"redemptionStatus": &types.AttributeValueMemberBOOL{Value: false},
					},
				},
			}
		}

		// Create the batch write input
		input := &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{
				"ads-qrcode-promotions-qe-ft-promocode": writeRequests,
			},
		}

		// Execute the batch write operation
		if _, err := dynamoClient.BatchWriteItem(ctx, input); err != nil {
			return fmt.Errorf("failed to batch write promocodes: %v", err)
		}
	}

	return nil
}