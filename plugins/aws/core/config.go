package core

// AWSConfig is the configuration for AWS plugin operations.
type AWSConfig struct {
	// Filters are optional AWS API filters (service-specific)
	Filters map[string][]string `json:"filters,omitempty" jsonschema:"description=AWS API filters"`

	// Service is the AWS service to query (iam, ec2, s3, rds, vpc, lambda)
	Service string `json:"service" jsonschema:"required,enum=iam,enum=ec2,enum=s3,enum=rds,enum=vpc,enum=lambda,description=AWS service to query"`

	// Operation is the service-specific operation to perform
	Operation string `json:"operation" jsonschema:"required,description=AWS API operation (e.g. describe_security_groups)"`

	// Region is the AWS region (defaults to AWS_REGION env var)
	Region string `json:"region,omitempty" jsonschema:"description=AWS region (defaults to AWS_REGION env var)"`

	// TimeoutSeconds is the request timeout (default 30)
	TimeoutSeconds int `json:"timeout_seconds,omitempty" jsonschema:"default=30,minimum=1,maximum=300,description=Request timeout in seconds"`
}

// AWSCredentials holds AWS authentication credentials.
type AWSCredentials struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string // Optional, for temporary credentials
	Region          string
}
