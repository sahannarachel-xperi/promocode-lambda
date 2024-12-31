package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-lambda-go/events"
)

type AdvertiserHandler interface {
	HandleCampaign(ctx context.Context, event events.S3EventRecord, data []byte, dynamoClient DynamoDBAPI) error
	HandlePromoCode(ctx context.Context, event events.S3EventRecord, data []byte, dynamoClient DynamoDBAPI) error
	HandleDeletion(ctx context.Context, event events.S3EventRecord, dynamoClient DynamoDBAPI, s3Client S3API) error
}

// Factory function to get the appropriate handler based on advertiser
func GetAdvertiserHandler(advertiser string) (AdvertiserHandler, error) {
	switch strings.ToLower(advertiser) {
	case "disney":
		return &DisneyHandler{}, nil
	// Add more advertisers here
	default:
		return nil, fmt.Errorf("unsupported advertiser: %s", advertiser)
	}
}