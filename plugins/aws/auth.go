//go:build wasip1

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

type AWSCredentials struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string // For temporary credentials
	Region          string
}

// getCredentials implements the standard AWS credential chain
func getCredentials(cfg AWSConfig) (*AWSCredentials, error) {
	creds := &AWSCredentials{}

	// 1. Explicit config (highest priority)
	if cfg.Region != "" {
		creds.Region = cfg.Region
	}

	// 2. Environment variables (via env capability)
	creds.AccessKeyID = os.Getenv("AWS_ACCESS_KEY_ID")
	creds.SecretAccessKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	creds.SessionToken = os.Getenv("AWS_SESSION_TOKEN")

	if creds.Region == "" {
		creds.Region = os.Getenv("AWS_REGION")
		if creds.Region == "" {
			creds.Region = os.Getenv("AWS_DEFAULT_REGION")
		}
	}

	// Validation
	if creds.AccessKeyID == "" || creds.SecretAccessKey == "" {
		return nil, fmt.Errorf("AWS credentials not found (set AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY)")
	}

	if creds.Region == "" {
		return nil, fmt.Errorf("AWS region not specified (set region in config or AWS_REGION env var)")
	}

	return creds, nil
}

// loadAWSConfig creates an aws.Config using the credentials and custom HTTP client
func loadAWSConfig(ctx context.Context, creds *AWSCredentials, httpClient *http.Client) (aws.Config, error) {
	return config.LoadDefaultConfig(ctx,
		config.WithRegion(creds.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			creds.AccessKeyID,
			creds.SecretAccessKey,
			creds.SessionToken,
		)),
		config.WithHTTPClient(httpClient),
	)
}
