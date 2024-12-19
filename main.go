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
	region   string              // AWS region to be set from environment variables
	dynamoDb *dynamodb.Client    // DynamoDB client to interact with DynamoDB tables
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
	// Load the AWS SDK configuration
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("unable to load SDK config: %v", err)
	}

	// Create an S3 client using the loaded configuration
	s3Client := s3.NewFromConfig(cfg)

	// Iterate through each S3 event record
	for _, record := range s3Event.Records {
		bucket := record.S3.Bucket.Name // Extract bucket name from the event
		key := record.S3.Object.Key    // Extract object key (file path) from the event

		// Retrieve the file from the S3 bucket
		data, err := utils.GetFileFromS3WithClient(ctx, s3Client, bucket, key)
		if err != nil {
			return fmt.Errorf("failed to get file from S3: %v", err)
		}

		// Determine the type of file based on its path and route to the appropriate handler
		isPromoCode := strings.Contains(key, "promocodes/") // Check if the file is related to promo codes
		if isPromoCode {
			// Handle promo code files
			if err := handlers.HandlePromoCode(ctx, record, data, dynamoDb); err != nil {
				return fmt.Errorf("failed to handle promo code: %v", err)
			}
		} else {
			// Handle campaign files
			if err := handlers.HandleCampaign(ctx, record, data, dynamoDb); err != nil {
				return fmt.Errorf("failed to handle campaign: %v", err)
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