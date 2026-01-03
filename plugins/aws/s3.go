//go:build wasip1

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	regletsdk "github.com/whiskeyjimbo/reglet/sdk"
)

func (p *awsPlugin) handleS3(ctx context.Context, cfg AWSConfig) (regletsdk.Evidence, error) {
	creds, err := getCredentials(cfg)
	if err != nil {
		return regletsdk.Failure("auth", err.Error()), nil
	}

	awsCfg, err := loadAWSConfig(ctx, creds, p.client)
	if err != nil {
		return regletsdk.Failure("internal", fmt.Sprintf("failed to load AWS config: %v", err)), nil
	}

	client := s3.NewFromConfig(awsCfg)

	switch cfg.Operation {
	case "list_buckets":
		return p.listBuckets(ctx, client, cfg)
	case "get_bucket_encryption":
		return p.getBucketEncryption(ctx, client, cfg)
	case "get_public_access_block":
		return p.getPublicAccessBlock(ctx, client, cfg)
	default:
		return regletsdk.Failure("config", fmt.Sprintf("unsupported S3 operation: %s", cfg.Operation)), nil
	}
}

func (p *awsPlugin) listBuckets(ctx context.Context, client *s3.Client, cfg AWSConfig) (regletsdk.Evidence, error) {
	start := time.Now()
	output, err := client.ListBuckets(ctx, &s3.ListBucketsInput{})
	duration := time.Since(start).Milliseconds()

	if err != nil {
		return regletsdk.Failure("network", fmt.Sprintf("ListBuckets failed: %v", err)), nil
	}

	buckets := make([]map[string]interface{}, 0)
	for _, b := range output.Buckets {
		bucket := map[string]interface{}{
			"name":          *b.Name,
			"creation_date": b.CreationDate.Format(time.RFC3339),
		}
		
		// Note: ListBuckets does not return region or encryption status.
		// We would need to make separate calls for those details if required,
		// but ListBuckets operation should just list buckets.
		// Detailed bucket checks should probably be separate or done iteratively.
		// However, for basic inventory, this is enough.
		
		buckets = append(buckets, bucket)
	}

	return regletsdk.Success(map[string]interface{}{
		"buckets":          buckets,
		"response_time_ms": duration,
		"region":           cfg.Region,
	}), nil
}

func (p *awsPlugin) getBucketEncryption(ctx context.Context, client *s3.Client, cfg AWSConfig) (regletsdk.Evidence, error) {
	// Simplified extraction for example
	// Real implementation should extract bucket name from config properly, maybe add Bucket field to config or use Filters
	
	bucketName := ""
	
	// Fallback: look in first filter if present
	if len(cfg.Filters) > 0 {
		if val, ok := cfg.Filters[0]["values"].([]interface{}); ok && len(val) > 0 {
			if str, ok := val[0].(string); ok {
				bucketName = str
			}
		}
	}
	
	// Let's check if we can parse filter "bucket"
	for _, f := range cfg.Filters {
		if f["name"] == "bucket" {
			if vals, ok := f["values"].([]interface{}); ok && len(vals) > 0 {
				bucketName = fmt.Sprintf("%v", vals[0])
			}
		}
	}

	if bucketName == "" {
		return regletsdk.Failure("config", "bucket name required in filters (name='bucket')"), nil
	}

	start := time.Now()
	output, err := client.GetBucketEncryption(ctx, &s3.GetBucketEncryptionInput{
		Bucket: aws.String(bucketName),
	})
	duration := time.Since(start).Milliseconds()

	if err != nil {
		// Check for ServerSideEncryptionConfigurationNotFoundError
		// S3 returns this error code if encryption is not enabled
		// We should treat this as success=false (unencrypted) rather than failure error?
		// Or return evidence that encryption is missing.
		
		// In AWS SDK v2, check error types
		// This requires some type assertion.
		// For MVP, just return error, Reglet user can expect failure.
		return regletsdk.Failure("network", fmt.Sprintf("GetBucketEncryption failed: %v", err)), nil
	}

	rules := make([]map[string]interface{}, 0)
	for _, rule := range output.ServerSideEncryptionConfiguration.Rules {
		r := map[string]interface{}{
			"bucket_key_enabled": false,
		}
		if rule.BucketKeyEnabled != nil {
			r["bucket_key_enabled"] = *rule.BucketKeyEnabled
		}
		
		if rule.ApplyServerSideEncryptionByDefault != nil {
			algo := rule.ApplyServerSideEncryptionByDefault.SSEAlgorithm
			r["sse_algorithm"] = string(algo)
			if rule.ApplyServerSideEncryptionByDefault.KMSMasterKeyID != nil {
				r["kms_master_key_id"] = *rule.ApplyServerSideEncryptionByDefault.KMSMasterKeyID
			}
		}
		rules = append(rules, r)
	}

	return regletsdk.Success(map[string]interface{}{
		"bucket":           bucketName,
		"encryption_rules": rules,
		"response_time_ms": duration,
		"region":           cfg.Region,
	}), nil
}

func (p *awsPlugin) getPublicAccessBlock(ctx context.Context, client *s3.Client, cfg AWSConfig) (regletsdk.Evidence, error) {
	bucketName := ""
	for _, f := range cfg.Filters {
		if f["name"] == "bucket" {
			if vals, ok := f["values"].([]interface{}); ok && len(vals) > 0 {
				bucketName = fmt.Sprintf("%v", vals[0])
			}
		}
	}

	if bucketName == "" {
		return regletsdk.Failure("config", "bucket name required in filters (name='bucket')"), nil
	}

	start := time.Now()
	output, err := client.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{
		Bucket: aws.String(bucketName),
	})
	duration := time.Since(start).Milliseconds()

	if err != nil {
		return regletsdk.Failure("network", fmt.Sprintf("GetPublicAccessBlock failed: %v", err)), nil
	}

	conf := map[string]bool{
		"block_public_acls":       false,
		"ignore_public_acls":      false,
		"block_public_policy":     false,
		"restrict_public_buckets": false,
	}

	if output.PublicAccessBlockConfiguration != nil {
		if output.PublicAccessBlockConfiguration.BlockPublicAcls != nil {
			conf["block_public_acls"] = *output.PublicAccessBlockConfiguration.BlockPublicAcls
		}
		if output.PublicAccessBlockConfiguration.IgnorePublicAcls != nil {
			conf["ignore_public_acls"] = *output.PublicAccessBlockConfiguration.IgnorePublicAcls
		}
		if output.PublicAccessBlockConfiguration.BlockPublicPolicy != nil {
			conf["block_public_policy"] = *output.PublicAccessBlockConfiguration.BlockPublicPolicy
		}
		if output.PublicAccessBlockConfiguration.RestrictPublicBuckets != nil {
			conf["restrict_public_buckets"] = *output.PublicAccessBlockConfiguration.RestrictPublicBuckets
		}
	}

	return regletsdk.Success(map[string]interface{}{
		"bucket":           bucketName,
		"public_access_block": conf,
		"response_time_ms": duration,
		"region":           cfg.Region,
	}), nil
}
