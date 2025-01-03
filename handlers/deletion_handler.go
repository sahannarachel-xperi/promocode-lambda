package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func HandleDeletion(ctx context.Context, event events.S3EventRecord, dynamoClient DynamoDBAPI, s3Client S3API) error {
	log.Printf("[INFO] Starting deletion process for: %s", event.S3.Object.Key)

	// Verify S3 deletion first
	_, err := s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(event.S3.Bucket.Name),
		Key:    aws.String(event.S3.Object.Key),
	})
	if err == nil {
		return fmt.Errorf("file still exists in S3: %s", event.S3.Object.Key)
	}

	// Parse and validate the S3 path structure
	// Expected format: qe-ft/campaigns/advertiser/campaign_id.json
	pathParts := strings.Split(event.S3.Object.Key, "/")
	if len(pathParts) < 4 {
		return fmt.Errorf("invalid path structure: %s", event.S3.Object.Key)
	}

	// Extract advertiser and campaign ID
	advertiser := pathParts[2]
	campaignIDWithExt := pathParts[3]

	// Remove the .json extension from campaign ID
	campaignID := strings.TrimSuffix(campaignIDWithExt, ".json")

	log.Printf("[DEBUG] Parsed campaign ID: %s for advertiser: %s", campaignID, advertiser)
	log.Printf("[INFO] Initiating campaign deletion for ID: %s from advertiser: %s", campaignID, advertiser)

	// Delete the campaign from DynamoDB
	log.Printf("[INFO] Deleting campaign with ID: %s", campaignID)
	_, err = dynamoClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String("ads-qrcode-promotions-qe-ft-campaign"),
			Key: map[string]types.AttributeValue{
				"campaignId": &types.AttributeValueMemberS{Value: campaignID},
			},
	})

	if err != nil {
		var notFoundErr *types.ResourceNotFoundException
		if errors.As(err, &notFoundErr) {
			log.Printf("[ERROR] Campaign with ID %s does not exist", campaignID)
			log.Printf("[INFO] Campaign %s already deleted", campaignID)
		} else {
			return fmt.Errorf("failed to delete campaign: %v", err)
		}
	}

	log.Printf("[INFO] Successfully deleted campaign: %s", campaignID)
	log.Printf("[INFO] Starting cleanup of associated promocodes")
	return cleanupPromocodes(ctx, campaignID, dynamoClient)
}

func handleCampaignDeletion(ctx context.Context, campaignID string, dynamoClient DynamoDBAPI) error {
	log.Printf("[INFO] Deleting campaign with ID: %s", campaignID)

	// Check if the campaign exists before attempting to delete
	getInput := &dynamodb.GetItemInput{
		TableName: aws.String("ads-qrcode-promotions-qe-ft-campaign"),
		Key: map[string]types.AttributeValue{
			"campaignId": &types.AttributeValueMemberS{Value: campaignID},
		},
	}

	result, err := dynamoClient.GetItem(ctx, getInput)
	if err != nil {
		log.Printf("[ERROR] Failed to check if campaign exists: %v", err)
		return fmt.Errorf("failed to check if campaign exists: %v", err)
	}

	if result.Item == nil {
		log.Printf("[ERROR] Campaign with ID %s does not exist", campaignID)
		return fmt.Errorf("campaign with ID %s does not exist", campaignID)
	}

	// Proceed to delete the campaign
	_, err = dynamoClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String("ads-qrcode-promotions-qe-ft-campaign"),
		Key: map[string]types.AttributeValue{
			"campaignId": &types.AttributeValueMemberS{Value: campaignID},
		},
	})
	if err != nil {
		log.Printf("[ERROR] Failed to delete campaign %s: %v", campaignID, err)
		return fmt.Errorf("failed to delete campaign: %v", err)
	}

	log.Printf("[INFO] Successfully deleted campaign: %s", campaignID)
	log.Printf("[INFO] Starting cleanup of associated promocodes")
	return cleanupPromocodes(ctx, campaignID, dynamoClient)
}

func handlePromocodeDeletion(ctx context.Context, campaignID string, dynamoClient DynamoDBAPI) error {
	return cleanupPromocodes(ctx, campaignID, dynamoClient)
}

func cleanupPromocodes(ctx context.Context, campaignID string, dynamoClient DynamoDBAPI) error {
	log.Printf("[INFO] Querying promocodes for campaign: %s", campaignID)

	var totalDeleted int
	var batchCount int

	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String("ads-qrcode-promotions-qe-ft-promocode"),
		KeyConditionExpression: aws.String("campaignId = :cid"),
		ProjectionExpression:   aws.String("campaignId, promocode"), // Only fetch key attributes
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":cid": &types.AttributeValueMemberS{Value: campaignID},
		},
	}

	for {
		result, err := dynamoClient.Query(ctx, queryInput)
		if err != nil {
			log.Printf("[ERROR] Failed to query promocodes: %v", err)
			return fmt.Errorf("failed to query promocodes: %v", err)
		}

		if len(result.Items) > 0 {
			batchCount++
			writeRequests := make([]types.WriteRequest, len(result.Items))
			for i, item := range result.Items {
				writeRequests[i] = types.WriteRequest{
					DeleteRequest: &types.DeleteRequest{
						Key: map[string]types.AttributeValue{
							"campaignId": item["campaignId"],
							"promocode":  item["promocode"],
						},
					},
				}
			}

			if err := processBatchDelete(ctx, writeRequests, dynamoClient); err != nil {
				log.Printf("[ERROR] Failed to process batch %d: %v", batchCount, err)
				return err
			}
			totalDeleted += len(result.Items)
			log.Printf("[INFO] Successfully processed batch %d, total deleted: %d", batchCount, totalDeleted)
		}

		if result.LastEvaluatedKey == nil {
			break
		}
		queryInput.ExclusiveStartKey = result.LastEvaluatedKey
	}

	log.Printf("[INFO] Completed promocode cleanup. Total deleted: %d", totalDeleted)
	return nil
}

func processBatchDelete(ctx context.Context, writeRequests []types.WriteRequest, dynamoClient DynamoDBAPI) error {
	input := &dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]types.WriteRequest{
			"ads-qrcode-promotions-qe-ft-promocode": writeRequests,
		},
	}

	// Implement exponential backoff for unprocessed items
	maxRetries := 3
	for retry := 0; retry < maxRetries; retry++ {
		result, err := dynamoClient.BatchWriteItem(ctx, input)
		if err != nil {
			log.Printf("Batch delete error on attempt %d: %v", retry+1, err)
			return err
		}

		if len(result.UnprocessedItems) == 0 {
			return nil
		}

		// If there are unprocessed items, retry with them
		input.RequestItems = result.UnprocessedItems
		time.Sleep(time.Duration(math.Pow(2, float64(retry))) * time.Second)
	}

	return fmt.Errorf("failed to process all items after %d retries", maxRetries)
}