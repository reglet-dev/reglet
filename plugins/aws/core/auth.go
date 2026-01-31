// Package core implements the shared infrastructure for the AWS plugin, including authentication, request signing, configuration parsing, error handling, and the HTTP client for interacting with AWS services.
package core

import (
	"fmt"
	"os"
)

// GetCredentials loads AWS credentials from environment variables.
// It follows the standard AWS credential chain (environment variables).
func GetCredentials(cfg *AWSConfig) (*AWSCredentials, error) {
	creds := &AWSCredentials{}

	// Load credentials from environment
	creds.AccessKeyID = os.Getenv("AWS_ACCESS_KEY_ID")
	creds.SecretAccessKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	creds.SessionToken = os.Getenv("AWS_SESSION_TOKEN") // Optional

	// Determine region: config > AWS_REGION > AWS_DEFAULT_REGION
	if cfg.Region != "" {
		creds.Region = cfg.Region
	} else {
		creds.Region = os.Getenv("AWS_REGION")
		if creds.Region == "" {
			creds.Region = os.Getenv("AWS_DEFAULT_REGION")
		}
	}

	// Validate required fields
	if creds.AccessKeyID == "" {
		return nil, fmt.Errorf("AWS_ACCESS_KEY_ID environment variable not set")
	}
	if creds.SecretAccessKey == "" {
		return nil, fmt.Errorf("AWS_SECRET_ACCESS_KEY environment variable not set")
	}
	if creds.Region == "" {
		return nil, fmt.Errorf("AWS region not specified (set region in config or AWS_REGION env var)")
	}

	return creds, nil
}
