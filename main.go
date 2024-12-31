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
	log.Printf("[INFO] Received S3 event with %d records", len(s3Event.Records))

	if err := coldStart(ctx); err != nil {
		return fmt.Errorf("failed to initialize AWS clients: %v", err)
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("unable to load SDK config: %v", err)
	}

	s3Client := s3.NewFromConfig(cfg)

	for _, record := range s3Event.Records {
		log.Printf("[INFO] Processing S3 event: %s, Key: %s", record.EventName, record.S3.Object.Key)

		// Extract advertiser from path
		pathParts := strings.Split(record.S3.Object.Key, "/")
		if len(pathParts) < 3 {
			log.Printf("[ERROR] Invalid path structure: %s", record.S3.Object.Key)
			continue
		}
		advertiser := pathParts[2]

		// Get the appropriate handler for the advertiser
		handler, err := handlers.GetAdvertiserHandler(advertiser)
		if err != nil {
			log.Printf("[ERROR] Failed to get handler for advertiser %s: %v", advertiser, err)
			return err
		}

		switch record.EventName {
		case "ObjectRemoved:Delete", "ObjectRemoved:DeleteMarkerCreated":
			log.Printf("[INFO] Processing deletion event for key: %s", record.S3.Object.Key)
			if err := handler.HandleDeletion(ctx, record, dynamoDb, s3Client); err != nil {
				log.Printf("[ERROR] Failed to handle deletion: %v", err)
				return fmt.Errorf("failed to handle deletion: %v", err)
			}
		case "ObjectCreated:Put", "ObjectCreated:Post":
			log.Printf("[INFO] Processing creation event for key: %s", record.S3.Object.Key)
			data, err := utils.GetFileFromS3WithClient(ctx, s3Client, record.S3.Bucket.Name, record.S3.Object.Key)
			if err != nil {
				log.Printf("[ERROR] Failed to get object from S3: %v", err)
				return fmt.Errorf("failed to get object from S3: %v", err)
			}

			if strings.Contains(record.S3.Object.Key, "/redemptions/") {
				log.Printf("[INFO] Processing redemption file: %s", record.S3.Object.Key)
				if err := handler.HandleRedemption(ctx, record, data, dynamoDb); err != nil {
					log.Printf("[ERROR] Failed to process responder file: %v", err)
					return fmt.Errorf("failed to handle responder file: %v", err)
				}
			} else if strings.Contains(record.S3.Object.Key, "/campaigns/") {
				log.Printf("[INFO] Processing campaign file: %s", record.S3.Object.Key)
				if err := handler.HandleCampaign(ctx, record, data, dynamoDb); err != nil {
					return fmt.Errorf("failed to handle campaign: %v", err)
				}
			} else if strings.Contains(record.S3.Object.Key, "/promocodes/") {
				log.Printf("[INFO] Processing promocode file: %s", record.S3.Object.Key)
				if err := handler.HandlePromoCode(ctx, record, data, dynamoDb); err != nil {
					return fmt.Errorf("failed to handle promocode: %v", err)
				}
			} else {
				log.Printf("[WARN] Unhandled file path: %s", record.S3.Object.Key)
			}
		default:
			log.Printf("[WARN] Unhandled event type: %s", record.EventName)
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