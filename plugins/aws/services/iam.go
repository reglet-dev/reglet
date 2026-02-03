// Package services implements AWS service-specific compliance checks.
package services

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"time"

	"github.com/reglet-dev/reglet-sdk/go/application/plugin"
	"github.com/reglet-dev/reglet/plugins/aws/core"
)

// IAMService handles IAM compliance checks.
// Auto-registers on package import via the var below.
type IAMService struct {
	plugin.Service `name:"iam" desc:"IAM identity and access management checks"`

	GetAccountSummary        plugin.Op[GetAccountSummaryInput, GetAccountSummaryOutput]               `desc:"Check if root account has MFA enabled" method:"GetAccountSummaryHandler"`
	GetAccountPasswordPolicy plugin.Op[GetAccountPasswordPolicyInput, GetAccountPasswordPolicyOutput] `desc:"Verify IAM password policy meets requirements" method:"GetAccountPasswordPolicyHandler"`
	ListAccessKeysWithUsage  plugin.Op[ListAccessKeysWithUsageInput, ListAccessKeysWithUsageOutput]   `desc:"Find access keys unused for specified days" method:"ListAccessKeysWithUsageHandler"`
}

// Auto-register on package import
func init() {
	// Register operation types with examples
	plugin.RegisterOp[GetAccountSummaryInput, GetAccountSummaryOutput]("GetAccountSummary",
		plugin.Example[GetAccountSummaryInput, GetAccountSummaryOutput]{
			Name:        "root_mfa_check",
			Description: "Check if root account has MFA enabled",
			Input:       GetAccountSummaryInput{},
			ExpectedOutput: &GetAccountSummaryOutput{
				RootMFAEnabled: true,
			},
		},
	)

	plugin.RegisterOp[GetAccountPasswordPolicyInput, GetAccountPasswordPolicyOutput]("GetAccountPasswordPolicy",
		plugin.Example[GetAccountPasswordPolicyInput, GetAccountPasswordPolicyOutput]{
			Name:        "password_policy_check",
			Description: "Verify password policy meets CIS benchmarks",
			Input:       GetAccountPasswordPolicyInput{},
			ExpectedOutput: &GetAccountPasswordPolicyOutput{
				PolicyExists: true,
			},
		},
		plugin.Example[GetAccountPasswordPolicyInput, GetAccountPasswordPolicyOutput]{
			Name:          "no_policy",
			Description:   "Error case: no password policy configured",
			Input:         GetAccountPasswordPolicyInput{},
			ExpectedError: "No password policy configured",
		},
	)

	plugin.RegisterOp[ListAccessKeysWithUsageInput, ListAccessKeysWithUsageOutput]("ListAccessKeysWithUsage",
		plugin.Example[ListAccessKeysWithUsageInput, ListAccessKeysWithUsageOutput]{
			Name:        "unused_keys_90_days",
			Description: "Find access keys unused for 90+ days",
			Input:       ListAccessKeysWithUsageInput{ThresholdDays: 90},
		},
		plugin.Example[ListAccessKeysWithUsageInput, ListAccessKeysWithUsageOutput]{
			Name:        "unused_keys_30_days",
			Description: "Find access keys unused for 30+ days",
			Input:       ListAccessKeysWithUsageInput{ThresholdDays: 30},
		},
	)

	plugin.MustRegisterService(core.Plugin, &IAMService{})
}

// =============================================================================
// Check 1: Root MFA Enabled
// =============================================================================

// GetAccountSummaryResponse represents the AWS GetAccountSummary response.
type GetAccountSummaryResponse struct {
	XMLName xml.Name `xml:"GetAccountSummaryResponse"`
	Result  struct {
		SummaryMap struct {
			Entry []struct {
				Key   string `xml:"key"`
				Value int    `xml:"value"`
			} `xml:"entry"`
		} `xml:"SummaryMap"`
	} `xml:"GetAccountSummaryResult"`
}

// GetAccountSummaryHandler checks if root account has MFA enabled.
func (s *IAMService) GetAccountSummaryHandler(ctx context.Context, in *GetAccountSummaryInput) (*GetAccountSummaryOutput, error) {
	client := plugin.GetClient[*core.AWSClient](ctx)

	// Call AWS API
	body, err := client.Call(ctx, "iam", "GetAccountSummary", nil)
	if err != nil {
		return nil, err
	}

	// Parse response
	var resp GetAccountSummaryResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse AWS response: %w", err)
	}

	// Extract summary values
	summary := make(map[string]int)
	for _, entry := range resp.Result.SummaryMap.Entry {
		summary[entry.Key] = entry.Value
	}

	return &GetAccountSummaryOutput{
		RootMFAEnabled:         summary["AccountMFAEnabled"] == 1,
		Users:                  summary["Users"],
		Groups:                 summary["Groups"],
		Roles:                  summary["Roles"],
		Policies:               summary["Policies"],
		MFADevices:             summary["MFADevices"],
		MFADevicesInUse:        summary["MFADevicesInUse"],
		AccessKeysPerUserQuota: summary["AccessKeysPerUserQuota"],
	}, nil
}

// =============================================================================
// Check 2: Password Policy Compliance
// =============================================================================

// GetAccountPasswordPolicyResponse represents the AWS response.
type GetAccountPasswordPolicyResponse struct {
	XMLName xml.Name `xml:"GetAccountPasswordPolicyResponse"`
	Result  struct {
		PasswordPolicy struct {
			MinimumPasswordLength      int  `xml:"MinimumPasswordLength"`
			MaxPasswordAge             int  `xml:"MaxPasswordAge"`
			PasswordReusePrevention    int  `xml:"PasswordReusePrevention"`
			RequireSymbols             bool `xml:"RequireSymbols"`
			RequireNumbers             bool `xml:"RequireNumbers"`
			RequireUppercaseCharacters bool `xml:"RequireUppercaseCharacters"`
			RequireLowercaseCharacters bool `xml:"RequireLowercaseCharacters"`
			AllowUsersToChangePassword bool `xml:"AllowUsersToChangePassword"`
			HardExpiry                 bool `xml:"HardExpiry"`
			ExpirePasswords            bool `xml:"ExpirePasswords"`
		} `xml:"PasswordPolicy"`
	} `xml:"GetAccountPasswordPolicyResult"`
}

// GetAccountPasswordPolicyHandler verifies IAM password policy.
func (s *IAMService) GetAccountPasswordPolicyHandler(ctx context.Context, in *GetAccountPasswordPolicyInput) (*GetAccountPasswordPolicyOutput, error) {
	client := plugin.GetClient[*core.AWSClient](ctx)

	// Call AWS API
	body, err := client.Call(ctx, "iam", "GetAccountPasswordPolicy", nil)
	if err != nil {
		// Check if no password policy exists
		var awsErr *core.AWSError
		if errors.As(err, &awsErr) && awsErr.Code == "NoSuchEntity" {
			return nil, fmt.Errorf("no password policy configured")
		}
		return nil, err
	}

	// Parse response
	var resp GetAccountPasswordPolicyResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse AWS response: %w", err)
	}

	policy := resp.Result.PasswordPolicy

	return &GetAccountPasswordPolicyOutput{
		PolicyExists: true,
		PasswordPolicy: &PasswordPolicyDetails{
			MinimumLength:           policy.MinimumPasswordLength,
			RequireSymbols:          policy.RequireSymbols,
			RequireNumbers:          policy.RequireNumbers,
			RequireUppercase:        policy.RequireUppercaseCharacters,
			RequireLowercase:        policy.RequireLowercaseCharacters,
			AllowUsersToChange:      policy.AllowUsersToChangePassword,
			MaxAgeDays:              policy.MaxPasswordAge,
			PasswordReusePrevention: policy.PasswordReusePrevention,
			HardExpiry:              policy.HardExpiry,
			ExpirePasswords:         policy.ExpirePasswords,
		},
	}, nil
}

// =============================================================================
// Check 3: Unused Access Keys (>N days)
// =============================================================================

// ListUsersResponse represents the AWS ListUsers response.
type ListUsersResponse struct {
	XMLName xml.Name `xml:"ListUsersResponse"`
	Result  struct {
		Marker string `xml:"Marker"`
		Users  struct {
			Member []struct {
				UserName string `xml:"UserName"`
				UserID   string `xml:"UserId"`
				Arn      string `xml:"Arn"`
			} `xml:"member"`
		} `xml:"Users"`
		IsTruncated bool `xml:"IsTruncated"`
	} `xml:"ListUsersResult"`
}

// ListAccessKeysResponse represents the AWS ListAccessKeys response.
type ListAccessKeysResponse struct {
	XMLName xml.Name `xml:"ListAccessKeysResponse"`
	Result  struct {
		AccessKeyMetadata struct {
			Member []struct {
				UserName    string `xml:"UserName"`
				AccessKeyID string `xml:"AccessKeyId"`
				Status      string `xml:"Status"`
				CreateDate  string `xml:"CreateDate"`
			} `xml:"member"`
		} `xml:"AccessKeyMetadata"`
	} `xml:"ListAccessKeysResult"`
}

// GetAccessKeyLastUsedResponse represents the AWS response.
type GetAccessKeyLastUsedResponse struct {
	XMLName xml.Name `xml:"GetAccessKeyLastUsedResponse"`
	Result  struct {
		AccessKeyLastUsed struct {
			LastUsedDate string `xml:"LastUsedDate"`
			ServiceName  string `xml:"ServiceName"`
			Region       string `xml:"Region"`
		} `xml:"AccessKeyLastUsed"`
	} `xml:"GetAccessKeyLastUsedResult"`
}

// ListAccessKeysWithUsageHandler finds unused access keys.
func (s *IAMService) ListAccessKeysWithUsageHandler(ctx context.Context, in *ListAccessKeysWithUsageInput) (*ListAccessKeysWithUsageOutput, error) {
	client := plugin.GetClient[*core.AWSClient](ctx)

	threshold := in.ThresholdDays
	if threshold == 0 {
		threshold = 90
	}

	// Step 1: List all users
	users, err := listAllUsers(ctx, client)
	if err != nil {
		return nil, err
	}

	// Step 2: For each user, list access keys and check usage
	var allKeys []AccessKeyInfo
	var unusedKeys []AccessKeyInfo
	now := time.Now()

	for _, userName := range users {
		keys, err := listAccessKeys(ctx, client, userName)
		if err != nil {
			// Log but continue
			fmt.Printf("Warning: Failed to list access keys for user %s: %v\n", userName, err)
			continue
		}

		for _, key := range keys {
			keyInfo := AccessKeyInfo{
				UserName:    userName,
				AccessKeyID: key.AccessKeyID,
				Status:      key.Status,
				Created:     key.CreateDate,
			}

			// Only check active keys for usage
			if key.Status == "Active" {
				lastUsed, err := getAccessKeyLastUsed(ctx, client, key.AccessKeyID)
				if err == nil && lastUsed != "" {
					keyInfo.LastUsed = lastUsed
					// Parse last used date
					if t, err := time.Parse(time.RFC3339, lastUsed); err == nil {
						daysSince := int(now.Sub(t).Hours() / 24)
						keyInfo.DaysSinceUsed = daysSince
						if daysSince > threshold {
							unusedKeys = append(unusedKeys, keyInfo)
						}
					}
				} else {
					// Key has never been used
					keyInfo.NeverUsed = true
					// Check creation date - if created > threshold days ago, it's unused
					if t, err := time.Parse(time.RFC3339, key.CreateDate); err == nil {
						daysSinceCreated := int(now.Sub(t).Hours() / 24)
						keyInfo.DaysSinceUsed = daysSinceCreated
						if daysSinceCreated > threshold {
							unusedKeys = append(unusedKeys, keyInfo)
						}
					}
				}
			}

			allKeys = append(allKeys, keyInfo)
		}
	}

	return &ListAccessKeysWithUsageOutput{
		TotalUsers:              len(users),
		TotalAccessKeys:         len(allKeys),
		AccessKeys:              allKeys,
		UnusedKeysOverThreshold: unusedKeys,
	}, nil
}

// listAllUsers returns all IAM user names (handles pagination).
func listAllUsers(ctx context.Context, client *core.AWSClient) ([]string, error) {
	var users []string
	var marker string

	for {
		params := make(map[string]string)
		if marker != "" {
			params["Marker"] = marker
		}
		params["MaxItems"] = "100"

		body, err := client.Call(ctx, "iam", "ListUsers", params)
		if err != nil {
			return nil, err
		}

		var resp ListUsersResponse
		if err := xml.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("failed to parse ListUsers response: %w", err)
		}

		for _, user := range resp.Result.Users.Member {
			users = append(users, user.UserName)
		}

		if !resp.Result.IsTruncated {
			break
		}
		marker = resp.Result.Marker
	}

	return users, nil
}

// listAccessKeys returns access keys for a user.
func listAccessKeys(ctx context.Context, client *core.AWSClient, userName string) ([]struct {
	AccessKeyID string
	Status      string
	CreateDate  string
}, error,
) {
	params := map[string]string{
		"UserName": userName,
	}

	body, err := client.Call(ctx, "iam", "ListAccessKeys", params)
	if err != nil {
		return nil, err
	}

	var resp ListAccessKeysResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	var keys []struct {
		AccessKeyID string
		Status      string
		CreateDate  string
	}
	for _, k := range resp.Result.AccessKeyMetadata.Member {
		keys = append(keys, struct {
			AccessKeyID string
			Status      string
			CreateDate  string
		}{
			AccessKeyID: k.AccessKeyID,
			Status:      k.Status,
			CreateDate:  k.CreateDate,
		})
	}

	return keys, nil
}

// getAccessKeyLastUsed returns when an access key was last used.
func getAccessKeyLastUsed(ctx context.Context, client *core.AWSClient, accessKeyID string) (string, error) {
	params := map[string]string{
		"AccessKeyId": accessKeyID,
	}

	body, err := client.Call(ctx, "iam", "GetAccessKeyLastUsed", params)
	if err != nil {
		return "", err
	}

	var resp GetAccessKeyLastUsedResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return "", err
	}

	return resp.Result.AccessKeyLastUsed.LastUsedDate, nil
}
