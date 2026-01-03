//go:build wasip1

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	regletsdk "github.com/whiskeyjimbo/reglet/sdk"
)

func (p *awsPlugin) handleIAM(ctx context.Context, cfg AWSConfig) (regletsdk.Evidence, error) {
	creds, err := getCredentials(cfg)
	if err != nil {
		return regletsdk.Failure("auth", err.Error()), nil
	}

	awsCfg, err := loadAWSConfig(ctx, creds, p.client)
	if err != nil {
		return regletsdk.Failure("internal", fmt.Sprintf("failed to load AWS config: %v", err)), nil
	}

	client := iam.NewFromConfig(awsCfg)

	switch cfg.Operation {
	case "list_users":
		return p.listUsers(ctx, client, cfg)
	case "get_account_password_policy":
		return p.getAccountPasswordPolicy(ctx, client, cfg)
	default:
		return regletsdk.Failure("config", fmt.Sprintf("unsupported IAM operation: %s", cfg.Operation)), nil
	}
}

func (p *awsPlugin) listUsers(ctx context.Context, client *iam.Client, cfg AWSConfig) (regletsdk.Evidence, error) {
	start := time.Now()
	output, err := client.ListUsers(ctx, &iam.ListUsersInput{})
	duration := time.Since(start).Milliseconds()

	if err != nil {
		return regletsdk.Failure("network", fmt.Sprintf("ListUsers failed: %v", err)), nil
	}

	users := make([]map[string]interface{}, 0)
	for _, u := range output.Users {
		user := map[string]interface{}{
			"user_id":   *u.UserId,
			"user_name": *u.UserName,
			"arn":       *u.Arn,
		}
		if u.CreateDate != nil {
			user["create_date"] = u.CreateDate.Format(time.RFC3339)
		}
		if u.PasswordLastUsed != nil {
			user["password_last_used"] = u.PasswordLastUsed.Format(time.RFC3339)
		}
		
		// For MFA, we need separate calls per user
		// In a real plugin we might want to fetch this optionally or in a separate operation
		// For MVP, we'll just list basic user info
		
		users = append(users, user)
	}

	return regletsdk.Success(map[string]interface{}{
		"users":            users,
		"response_time_ms": duration,
	}), nil
}

func (p *awsPlugin) getAccountPasswordPolicy(ctx context.Context, client *iam.Client, cfg AWSConfig) (regletsdk.Evidence, error) {
	start := time.Now()
	output, err := client.GetAccountPasswordPolicy(ctx, &iam.GetAccountPasswordPolicyInput{})
	duration := time.Since(start).Milliseconds()

	if err != nil {
		// If no password policy is set, AWS returns a 404
		// We should handle this as "not found" but maybe successful observation
		return regletsdk.Failure("network", fmt.Sprintf("GetAccountPasswordPolicy failed: %v", err)), nil
	}

	policy := map[string]interface{}{
		"allow_users_to_change_password": output.PasswordPolicy.AllowUsersToChangePassword,
		"expire_passwords":               output.PasswordPolicy.ExpirePasswords,
		"hard_expiry":                    output.PasswordPolicy.HardExpiry,
		"minimum_password_length":        *output.PasswordPolicy.MinimumPasswordLength,
		"require_lowercase_characters":   output.PasswordPolicy.RequireLowercaseCharacters,
		"require_numbers":                output.PasswordPolicy.RequireNumbers,
		"require_symbols":                output.PasswordPolicy.RequireSymbols,
		"require_uppercase_characters":   output.PasswordPolicy.RequireUppercaseCharacters,
	}
	if output.PasswordPolicy.MaxPasswordAge != nil {
		policy["max_password_age"] = *output.PasswordPolicy.MaxPasswordAge
	}
	if output.PasswordPolicy.PasswordReusePrevention != nil {
		policy["password_reuse_prevention"] = *output.PasswordPolicy.PasswordReusePrevention
	}

	return regletsdk.Success(map[string]interface{}{
		"password_policy":  policy,
		"response_time_ms": duration,
	}), nil
}
