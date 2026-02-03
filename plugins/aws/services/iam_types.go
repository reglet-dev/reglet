package services

// =============================================================================
// GetAccountSummary - Root MFA Check
// =============================================================================

// GetAccountSummaryInput defines the input for the GetAccountSummary operation.
// This operation requires no additional input beyond AWS credentials.
type GetAccountSummaryInput struct {
	// No additional fields - AWS credentials come from context
}

// GetAccountSummaryOutput contains the AWS account summary metrics.
type GetAccountSummaryOutput struct {
	// RootMFAEnabled indicates if the root account has MFA enabled
	RootMFAEnabled bool `json:"root_mfa_enabled" jsonschema:"description=Whether root account has MFA enabled"`

	// Users is the count of IAM users
	Users int `json:"users" jsonschema:"description=Number of IAM users"`

	// Groups is the count of IAM groups
	Groups int `json:"groups" jsonschema:"description=Number of IAM groups"`

	// Roles is the count of IAM roles
	Roles int `json:"roles" jsonschema:"description=Number of IAM roles"`

	// Policies is the count of customer-managed policies
	Policies int `json:"policies" jsonschema:"description=Number of customer-managed policies"`

	// MFADevices is the count of MFA devices
	MFADevices int `json:"mfa_devices" jsonschema:"description=Number of MFA devices"`

	// MFADevicesInUse is the count of MFA devices in use
	MFADevicesInUse int `json:"mfa_devices_in_use" jsonschema:"description=Number of MFA devices in use"`

	// AccessKeysPerUserQuota is the quota for access keys per user
	AccessKeysPerUserQuota int `json:"access_keys_per_user_quota" jsonschema:"description=Access keys per user quota"`
}

// =============================================================================
// GetAccountPasswordPolicy - Password Policy Check
// =============================================================================

// GetAccountPasswordPolicyInput defines the input for password policy check.
type GetAccountPasswordPolicyInput struct {
	// No additional fields
}

// GetAccountPasswordPolicyOutput contains the IAM password policy details.
type GetAccountPasswordPolicyOutput struct {
	// PolicyExists indicates if a password policy is configured
	PolicyExists bool `json:"policy_exists" jsonschema:"description=Whether a password policy exists"`

	// PasswordPolicy contains the policy details (nil if no policy)
	PasswordPolicy *PasswordPolicyDetails `json:"password_policy,omitempty" jsonschema:"description=Password policy configuration"`
}

// PasswordPolicyDetails contains the password policy configuration.
type PasswordPolicyDetails struct {
	MinimumLength           int  `json:"minimum_length" jsonschema:"description=Minimum password length"`
	RequireSymbols          bool `json:"require_symbols" jsonschema:"description=Whether symbols are required"`
	RequireNumbers          bool `json:"require_numbers" jsonschema:"description=Whether numbers are required"`
	RequireUppercase        bool `json:"require_uppercase" jsonschema:"description=Whether uppercase letters are required"`
	RequireLowercase        bool `json:"require_lowercase" jsonschema:"description=Whether lowercase letters are required"`
	AllowUsersToChange      bool `json:"allow_users_to_change" jsonschema:"description=Whether users can change their password"`
	MaxAgeDays              int  `json:"max_age_days" jsonschema:"description=Maximum password age in days"`
	PasswordReusePrevention int  `json:"password_reuse_prevention" jsonschema:"description=Number of previous passwords to prevent reuse"`
	HardExpiry              bool `json:"hard_expiry" jsonschema:"description=Whether passwords expire immediately"`
	ExpirePasswords         bool `json:"expire_passwords" jsonschema:"description=Whether password expiration is enabled"`
}

// =============================================================================
// ListAccessKeysWithUsage - Unused Access Keys Check
// =============================================================================

// ListAccessKeysWithUsageInput defines the input for access keys usage check.
type ListAccessKeysWithUsageInput struct {
	// ThresholdDays is the number of days after which a key is considered unused (default: 90)
	ThresholdDays int `json:"threshold_days,omitempty" jsonschema:"default=90,minimum=1,description=Days of inactivity to consider key unused"`
}

// ListAccessKeysWithUsageOutput contains access key usage information.
type ListAccessKeysWithUsageOutput struct {
	// TotalUsers is the count of IAM users
	TotalUsers int `json:"total_users" jsonschema:"description=Total number of IAM users"`

	// TotalAccessKeys is the count of all access keys
	TotalAccessKeys int `json:"total_access_keys" jsonschema:"description=Total number of access keys"`

	// AccessKeys contains details of all access keys
	AccessKeys []AccessKeyInfo `json:"access_keys" jsonschema:"description=All access key details"`

	// UnusedKeysOverThreshold contains keys unused beyond the threshold
	UnusedKeysOverThreshold []AccessKeyInfo `json:"unused_keys_over_threshold" jsonschema:"description=Access keys unused beyond threshold"`
}

// AccessKeyInfo holds access key usage information.
type AccessKeyInfo struct {
	UserName      string `json:"user_name" jsonschema:"description=IAM user name"`
	AccessKeyID   string `json:"access_key_id" jsonschema:"description=Access key ID"`
	Status        string `json:"status" jsonschema:"description=Key status (Active/Inactive)"`
	Created       string `json:"created" jsonschema:"description=Key creation date"`
	LastUsed      string `json:"last_used,omitempty" jsonschema:"description=Last used date"`
	DaysSinceUsed int    `json:"days_since_used,omitempty" jsonschema:"description=Days since last use"`
	NeverUsed     bool   `json:"never_used,omitempty" jsonschema:"description=Whether key was never used"`
}
