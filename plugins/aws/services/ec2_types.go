package services

// =============================================================================
// DescribeSecurityGroups - Open SSH/RDP Check
// =============================================================================

// DescribeSecurityGroupsInput defines the input for security groups check.
type DescribeSecurityGroupsInput struct {
	// Filters are optional AWS API filters
	Filters map[string][]string `json:"filters,omitempty" jsonschema:"description=AWS API filters (e.g. vpc-id)"`

	// Region overrides the default AWS region
	Region string `json:"region,omitempty" jsonschema:"description=AWS region (defaults to configured region)"`
}

// DescribeSecurityGroupsOutput contains security group analysis.
type DescribeSecurityGroupsOutput struct {
	// Region is the AWS region queried
	Region string `json:"region" jsonschema:"description=AWS region queried"`

	// TotalGroups is the count of security groups
	TotalGroups int `json:"total_groups" jsonschema:"description=Total number of security groups"`

	// SecurityGroups contains all security group details
	SecurityGroups []SecurityGroupInfo `json:"security_groups" jsonschema:"description=All security group details"`

	// OpenSSHGroups contains groups with open SSH (port 22) to 0.0.0.0/0
	OpenSSHGroups []SecurityGroupInfo `json:"open_ssh_groups" jsonschema:"description=Security groups with open SSH to internet"`

	// OpenRDPGroups contains groups with open RDP (port 3389) to 0.0.0.0/0
	OpenRDPGroups []SecurityGroupInfo `json:"open_rdp_groups" jsonschema:"description=Security groups with open RDP to internet"`
}

// SecurityGroupInfo holds security group details.
type SecurityGroupInfo struct {
	GroupID      string            `json:"group_id" jsonschema:"description=Security group ID"`
	GroupName    string            `json:"group_name" jsonschema:"description=Security group name"`
	VpcID        string            `json:"vpc_id" jsonschema:"description=VPC ID"`
	Description  string            `json:"description" jsonschema:"description=Security group description"`
	Tags         map[string]string `json:"tags" jsonschema:"description=Resource tags"`
	IngressRules []IngressRule     `json:"ingress_rules" jsonschema:"description=Inbound rules"`
}

// IngressRule holds a security group inbound rule.
type IngressRule struct {
	Protocol       string   `json:"protocol" jsonschema:"description=IP protocol (tcp, udp, -1 for all)"`
	FromPort       int      `json:"from_port" jsonschema:"description=Start of port range"`
	ToPort         int      `json:"to_port" jsonschema:"description=End of port range"`
	CidrBlocks     []string `json:"cidr_blocks" jsonschema:"description=IPv4 CIDR blocks"`
	Ipv6CidrBlocks []string `json:"ipv6_cidr_blocks,omitempty" jsonschema:"description=IPv6 CIDR blocks"`
	Description    string   `json:"description,omitempty" jsonschema:"description=Rule description"`
}

// =============================================================================
// DescribeInstancesMetadata - IMDSv2 Check
// =============================================================================

// DescribeInstancesMetadataInput defines the input for IMDSv2 check.
type DescribeInstancesMetadataInput struct {
	// Filters are optional AWS API filters
	Filters map[string][]string `json:"filters,omitempty" jsonschema:"description=AWS API filters (e.g. tag:Environment)"`

	// Region overrides the default AWS region
	Region string `json:"region,omitempty" jsonschema:"description=AWS region (defaults to configured region)"`
}

// DescribeInstancesMetadataOutput contains instance metadata analysis.
type DescribeInstancesMetadataOutput struct {
	// Region is the AWS region queried
	Region string `json:"region" jsonschema:"description=AWS region queried"`

	// TotalInstances is the count of running instances
	TotalInstances int `json:"total_instances" jsonschema:"description=Total number of running instances"`

	// Instances contains all instance details
	Instances []InstanceMetadataInfo `json:"instances" jsonschema:"description=All instance metadata details"`

	// NonCompliantInstances contains instances not enforcing IMDSv2
	NonCompliantInstances []InstanceMetadataInfo `json:"non_compliant_instances" jsonschema:"description=Instances not enforcing IMDSv2"`
}

// InstanceMetadataInfo holds instance metadata settings.
type InstanceMetadataInfo struct {
	InstanceID      string            `json:"instance_id" jsonschema:"description=EC2 instance ID"`
	InstanceType    string            `json:"instance_type" jsonschema:"description=Instance type"`
	State           string            `json:"state" jsonschema:"description=Instance state"`
	Tags            map[string]string `json:"tags" jsonschema:"description=Resource tags"`
	MetadataOptions MetadataOptions   `json:"metadata_options" jsonschema:"description=Metadata service options"`
	IMDSv2Enforced  bool              `json:"imdsv2_enforced" jsonschema:"description=Whether IMDSv2 is required"`
}

// MetadataOptions holds IMDS configuration.
type MetadataOptions struct {
	HTTPTokens   string `json:"http_tokens" jsonschema:"description=Token requirement (optional/required)"`
	HTTPEndpoint string `json:"http_endpoint" jsonschema:"description=Metadata endpoint status"`
}
