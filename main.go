package main

import (
	"context"
	"fmt"
	"os"
	"promocode-lambda/handlers"
	"promocode-lambda/utils"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	log "github.com/sirupsen/logrus"
)

// Global variables for AWS resources and configuration
var (
	region   string           // AWS region to be set from environment variables
	dynamoDb *dynamodb.Client // DynamoDB client to interact with DynamoDB tables
)

func coldStart(ctx context.Context) error {
	// Load AWS region from environment variable
	region = os.Getenv("AWS_REGION")
	if region == "" {
		return fmt.Errorf("could not find aws region in environment")
	}

	// Load the AWS default configuration
	awscfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %v", err)
	}

	// Initialize DynamoDB client using the loaded configuration
	dynamoDb = dynamodb.NewFromConfig(awscfg)

	// Configure log level based on the environment variable
	setLogLevel()
	return nil
}

// setLogLevel configures the logging level based on environment variable
func setLogLevel() {
	logLevel := os.Getenv("LOG_LEVEL") // Fetch logging level from environment variable
	switch logLevel {
	case "DEBUG":
		log.SetLevel(log.DebugLevel)
	case "INFO":
		log.SetLevel(log.InfoLevel)
	case "WARN":
		log.SetLevel(log.WarnLevel)
	case "ERROR":
		log.SetLevel(log.ErrorLevel)
	default:
		log.SetLevel(log.DebugLevel) // Default to DEBUG level if no valid log level is specified
	}
}

// handleRequest processes S3 events for campaign and promocode files
func handleRequest(ctx context.Context, s3Event events.S3Event) error {
	if err := coldStart(ctx); err != nil {
		return fmt.Errorf("failed to initialize AWS clients: %v", err)
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("unable to load SDK config: %v", err)
	}

	s3Client := s3.NewFromConfig(cfg)

	for _, record := range s3Event.Records {
		// Extract advertiser from path
		// Expected path format: qe-ft/[type]/[advertiser]/[campaignId]/...
		pathParts := strings.Split(record.S3.Object.Key, "/")
		if len(pathParts) < 4 {
			return fmt.Errorf("invalid path structure: %s, expected qe-ft/type/advertiser/campaignId/...", record.S3.Object.Key)
		}

		// Get advertiser name from the correct path segment
		advertiser := pathParts[2]
		// Remove any campaign ID or other suffixes from advertiser name
		advertiser = strings.Split(advertiser, "-")[0] // This will get "disney" from "disney1" or "disney-campaign1"

		// Get appropriate handler for the advertiser
		handler, err := handlers.GetAdvertiserHandler(advertiser)
		if err != nil {
			return fmt.Errorf("advertiser handler error: %v (path: %s)", err, record.S3.Object.Key)
		}

		switch record.EventName {
		case "ObjectRemoved:Delete", "ObjectRemoved:DeleteMarkerCreated":
			if err := handler.HandleDeletion(ctx, record, dynamoDb, s3Client); err != nil {
				return fmt.Errorf("failed to handle deletion: %v", err)
			}
		case "ObjectCreated:Put", "ObjectCreated:Post":
			data, err := utils.GetFileFromS3WithClient(ctx, s3Client, record.S3.Bucket.Name, record.S3.Object.Key)
			if err != nil {
				return fmt.Errorf("failed to get object from S3: %v", err)
			}

			if strings.Contains(record.S3.Object.Key, "/campaigns/") {
				if err := handler.HandleCampaign(ctx, record, data, dynamoDb); err != nil {
					return fmt.Errorf("failed to handle campaign: %v", err)
				}
			} else if strings.Contains(record.S3.Object.Key, "/promocodes/") {
				if err := handler.HandlePromoCode(ctx, record, data, dynamoDb); err != nil {
					return fmt.Errorf("failed to handle promocode: %v", err)
				}
			}
		}
	}
	return nil
}

func main() {
	// Create a new context
	ctx := context.Background()

	// Perform cold-start initialization
	err := coldStart(ctx)
	if err != nil {
		panic(err) // Terminate the application if initialization fails
	}

	// Start the Lambda handler
	lambda.Start(handleRequest)
}