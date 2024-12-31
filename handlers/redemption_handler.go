package handlers

import (
    "context"
    "encoding/csv"
    "fmt"
    "io"
    "log"
    "strings"

    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
    "promocode-lambda/utils"
)

func HandleRedemption(ctx context.Context, event events.S3EventRecord, data []byte, dynamoClient DynamoDBAPI) error {
    log.Printf("[INFO] Processing responder file: %s", event.S3.Object.Key)

    // Validate that we received non-empty data
    if len(data) == 0 {
        return fmt.Errorf("empty redemption data received")
    }

    // Ensure we're processing a CSV file
    if !strings.HasSuffix(event.S3.Object.Key, ".csv") {
        return fmt.Errorf("invalid file type, expected CSV file for redemptions, got: %s", event.S3.Object.Key)
    }

    // Parse CSV data
    reader := csv.NewReader(strings.NewReader(string(data)))
    reader.Comma = ',' // Set comma as delimiter

    // Read and skip header
    _, err := reader.Read()
    if err != nil {
        return fmt.Errorf("failed to read CSV header: %v", err)
    }

    // Process each record
    for {
        record, err := reader.Read()
        if err == io.EOF {
            break
        }
        if err != nil {
            log.Printf("[WARN] Error reading CSV record: %v", err)
            continue
        }

        // Extract fields from CSV record
        if len(record) < 5 {
            log.Printf("[WARN] Invalid record format: %v", record)
            continue
        }

        campaignCode := record[1]    // campaign_code field
        redemptionCode := record[3]  // redemption_code field

        // First, check if the promo code exists for this campaign
        getInput := &dynamodb.GetItemInput{
            TableName: aws.String("ads-qrcode-promotions-qe-ft-promocode"),
            Key: map[string]types.AttributeValue{
                "campaignId": &types.AttributeValueMemberS{Value: campaignCode},
                "promocode":  &types.AttributeValueMemberS{Value: redemptionCode},
            },
        }

        result, err := dynamoClient.GetItem(ctx, getInput)
        if err != nil {
            log.Printf("[WARN] Failed to check promo code existence: %v", err)
            continue
        }

        // If the item doesn't exist, skip it
        if result.Item == nil {
            log.Printf("[WARN] Promo code %s not found for campaign %s", redemptionCode, campaignCode)
            continue
        }

        // Update the promo code redemption status
        updateInput := &dynamodb.UpdateItemInput{
            TableName: aws.String("ads-qrcode-promotions-qe-ft-promocode"),
            Key: map[string]types.AttributeValue{
                "campaignId": &types.AttributeValueMemberS{Value: campaignCode},
                "promocode":  &types.AttributeValueMemberS{Value: redemptionCode},
            },
            UpdateExpression: aws.String("SET redemptionStatus = :status"),
            ExpressionAttributeValues: map[string]types.AttributeValue{
                ":status": &types.AttributeValueMemberBOOL{Value: true},
            },
            ReturnValues: types.ReturnValueNone,
        }

        if err := utils.RetryWithBackoff(ctx, func() error {
            _, err := dynamoClient.UpdateItem(ctx, updateInput)
            return err
        }); err != nil {
            log.Printf("[WARN] Failed to update promo code %s for campaign %s: %v",
                redemptionCode, campaignCode, err)
            continue
        }

        log.Printf("[INFO] Successfully marked promo code %s as redeemed for campaign %s",
            redemptionCode, campaignCode)
    }

    return nil
}