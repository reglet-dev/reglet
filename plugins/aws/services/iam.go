// Package services implements AWS service-specific compliance checks.
package services

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"time"

	"github.com/reglet-dev/reglet-sdk/go/application/plugin"
	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
	"github.com/reglet-dev/reglet/plugins/aws/core"
)

// IAMService handles IAM compliance checks.
// Auto-registers on package import via the var below.
// IAMService handles IAM compliance checks.
// Auto-registers on package import via the var below.
type IAMService struct {
	plugin.Service `name:"iam" desc:"IAM identity and access management checks"`

	GetAccountSummary        plugin.Op `desc:"Check if root account has MFA enabled" method:"GetAccountSummaryHandler"`
	GetAccountPasswordPolicy plugin.Op `desc:"Verify IAM password policy meets requirements" method:"GetAccountPasswordPolicyHandler"`
	ListAccessKeysWithUsage  plugin.Op `desc:"Find access keys unused for 90+ days" method:"ListAccessKeysWithUsageHandler"`
}

// Auto-register on package import
func init() {
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

func (s *IAMService) GetAccountSummaryHandler(ctx context.Context, req *plugin.Request) (*entities.Result, error) {
	client := req.Client.(*core.AWSClient)

	// Call AWS API
	body, err := client.Call(ctx, "iam", "GetAccountSummary", nil)
	if err != nil {
		return nil, err
	}

	// Parse response
	var resp GetAccountSummaryResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return entities.ResultErrorPtr("internal", fmt.Sprintf("Failed to parse AWS response: %v", err)), nil
	}

	// Extract summary values
	summary := make(map[string]int)
	for _, entry := range resp.Result.SummaryMap.Entry {
		summary[entry.Key] = entry.Value
	}

	// Check root MFA status
	// AccountMFAEnabled = 1 means root account has MFA enabled
	rootMFAEnabled := summary["AccountMFAEnabled"] == 1

	data := map[string]any{
		"service":                    "iam",
		"operation":                  "get_account_summary",
		"root_mfa_enabled":           rootMFAEnabled,
		"users":                      summary["Users"],
		"groups":                     summary["Groups"],
		"roles":                      summary["Roles"],
		"policies":                   summary["Policies"],
		"mfa_devices":                summary["MFADevices"],
		"mfa_devices_in_use":         summary["MFADevicesInUse"],
		"access_keys_per_user_quota": summary["AccessKeysPerUserQuota"],
	}

	if rootMFAEnabled {
		return entities.ResultSuccessPtr("Root account has MFA enabled", data), nil
	}
	return entities.ResultFailurePtr("Root account does NOT have MFA enabled", data), nil
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

func (s *IAMService) GetAccountPasswordPolicyHandler(ctx context.Context, req *plugin.Request) (*entities.Result, error) {
	client := req.Client.(*core.AWSClient)

	// Call AWS API
	body, err := client.Call(ctx, "iam", "GetAccountPasswordPolicy", nil)
	if err != nil {
		// Check if no password policy exists
		var awsErr *core.AWSError
		if errors.As(err, &awsErr) && awsErr.Code == "NoSuchEntity" {
			return entities.ResultFailurePtr("No password policy configured", map[string]any{
				"service":         "iam",
				"operation":       "get_account_password_policy",
				"policy_exists":   false,
				"password_policy": nil,
			}), nil
		}
		return nil, err
	}

	// Parse response
	var resp GetAccountPasswordPolicyResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return entities.ResultErrorPtr("internal", fmt.Sprintf("Failed to parse AWS response: %v", err)), nil
	}

	policy := resp.Result.PasswordPolicy

	// Build policy data
	policyData := map[string]any{
		"minimum_length":            policy.MinimumPasswordLength,
		"require_symbols":           policy.RequireSymbols,
		"require_numbers":           policy.RequireNumbers,
		"require_uppercase":         policy.RequireUppercaseCharacters,
		"require_lowercase":         policy.RequireLowercaseCharacters,
		"allow_users_to_change":     policy.AllowUsersToChangePassword,
		"max_age_days":              policy.MaxPasswordAge,
		"password_reuse_prevention": policy.PasswordReusePrevention,
		"hard_expiry":               policy.HardExpiry,
		"expire_passwords":          policy.ExpirePasswords,
	}

	data := map[string]any{
		"service":         "iam",
		"operation":       "get_account_password_policy",
		"policy_exists":   true,
		"password_policy": policyData,
	}

	// Check compliance (CIS Benchmark recommendations)
	// - Minimum 14 characters
	// - Require symbols, numbers, uppercase, lowercase
	// - Password expiration enabled
	compliant := policy.MinimumPasswordLength >= 14 &&
		policy.RequireSymbols &&
		policy.RequireNumbers &&
		policy.RequireUppercaseCharacters &&
		policy.RequireLowercaseCharacters

	if compliant {
		return entities.ResultSuccessPtr("Password policy meets security requirements", data), nil
	}
	return entities.ResultFailurePtr("Password policy does not meet security requirements", data), nil
}

// =============================================================================
// Check 3: Unused Access Keys (>90 days)
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

// AccessKeyInfo holds access key usage information.
type AccessKeyInfo struct {
	UserName      string `json:"user_name"`
	AccessKeyID   string `json:"access_key_id"`
	Status        string `json:"status"`
	Created       string `json:"created"`
	LastUsed      string `json:"last_used,omitempty"`
	DaysSinceUsed int    `json:"days_since_used,omitempty"`
	NeverUsed     bool   `json:"never_used,omitempty"`
}

func (s *IAMService) ListAccessKeysWithUsageHandler(ctx context.Context, req *plugin.Request) (*entities.Result, error) {
	client := req.Client.(*core.AWSClient)

	// Step 1: List all users
	users, err := listAllUsers(ctx, client)
	if err != nil {
		return nil, err
	}

	// Step 2: For each user, list access keys and check usage
	var allKeys []AccessKeyInfo
	var unusedKeysOver90Days []AccessKeyInfo
	now := time.Now()

	for _, userName := range users {
		keys, err := listAccessKeys(ctx, client, userName)
		if err != nil {
			// Log but continue
			continue
		}

		for _, key := range keys {
			keyInfo := AccessKeyInfo{
				UserName:    userName,
				AccessKeyID: key.AccessKeyID,
				Status:      key.Status,
				Created:     key.CreateDate,
			}

			// Only check active keys
			if key.Status == "Active" {
				lastUsed, err := getAccessKeyLastUsed(ctx, client, key.AccessKeyID)
				if err == nil && lastUsed != "" {
					keyInfo.LastUsed = lastUsed
					// Parse last used date
					if t, err := time.Parse(time.RFC3339, lastUsed); err == nil {
						daysSince := int(now.Sub(t).Hours() / 24)
						keyInfo.DaysSinceUsed = daysSince
						if daysSince > 90 {
							unusedKeysOver90Days = append(unusedKeysOver90Days, keyInfo)
						}
					}
				} else {
					// Key has never been used
					keyInfo.NeverUsed = true
					// Check creation date - if created > 90 days ago, it's unused
					if t, err := time.Parse(time.RFC3339, key.CreateDate); err == nil {
						daysSinceCreated := int(now.Sub(t).Hours() / 24)
						if daysSinceCreated > 90 {
							keyInfo.DaysSinceUsed = daysSinceCreated
							unusedKeysOver90Days = append(unusedKeysOver90Days, keyInfo)
						}
					}
				}
			}

			allKeys = append(allKeys, keyInfo)
		}
	}

	data := map[string]any{
		"service":                  "iam",
		"operation":                "list_access_keys_with_usage",
		"total_users":              len(users),
		"total_access_keys":        len(allKeys),
		"access_keys":              allKeys,
		"unused_keys_over_90_days": unusedKeysOver90Days,
	}

	if len(unusedKeysOver90Days) == 0 {
		return entities.ResultSuccessPtr("No access keys unused for more than 90 days", data), nil
	}
	return entities.ResultFailurePtr(
		fmt.Sprintf("%d access key(s) unused for more than 90 days", len(unusedKeysOver90Days)),
		data,
	), nil
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
