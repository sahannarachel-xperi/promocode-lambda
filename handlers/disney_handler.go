package handlers

import (
    "context"
    "github.com/aws/aws-lambda-go/events"
)

type DisneyHandler struct{}


func (h *DisneyHandler) HandleCampaign(ctx context.Context, event events.S3EventRecord, data []byte, dynamoClient DynamoDBAPI) error {
    // Reference existing implementation from handlers/campaign_handler.go
    return HandleCampaign(ctx, event, data, dynamoClient)
}

func (h *DisneyHandler) HandlePromoCode(ctx context.Context, event events.S3EventRecord, data []byte, dynamoClient DynamoDBAPI) error {
    // Reference existing implementation from handlers/promocode_handler.go
    return HandlePromoCode(ctx, event, data, dynamoClient)
}

func (h *DisneyHandler) HandleDeletion(ctx context.Context, event events.S3EventRecord, dynamoClient DynamoDBAPI, s3Client S3API) error {
    // Reference existing implementation from handlers/deletion_handler.go
    return HandleDeletion(ctx, event, dynamoClient, s3Client)
}

func (h *DisneyHandler) HandleRedemption(ctx context.Context, event events.S3EventRecord, data []byte, dynamoClient DynamoDBAPI) error {
    return HandleRedemption(ctx, event, data, dynamoClient)
}