package main

import (
	"fmt"

	"github.com/whiskeyjimbo/reglet/internal/domain/entities"
)

type ProfileGenerator struct {
	framework string
	region    string
}

func NewProfileGenerator(framework, region string) *ProfileGenerator {
	return &ProfileGenerator{framework: framework, region: region}
}

func (g *ProfileGenerator) Generate(resources *ScanResult) (*entities.Profile, error) {
	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{
			Name:        "aws-generated-profile",
			Version:     "1.0.0",
			Description: fmt.Sprintf("Auto-generated profile for AWS region %s", g.region),
		},
		Controls: entities.ControlsSection{
			Defaults: &entities.ControlDefaults{
				Severity: "warning",
				Tags:     []string{"aws", "auto-generated"},
			},
			Items: []entities.Control{},
		},
	}

	// Generate EC2 controls
	for _, sgID := range resources.EC2.SecurityGroups {
		profile.Controls.Items = append(profile.Controls.Items, entities.Control{
			ID:          fmt.Sprintf("aws-ec2-sg-%s-ssh", sgID),
			Name:        fmt.Sprintf("Security Group %s SSH Access", sgID),
			Description: "Ensure security group does not allow open SSH access",
			Observations: []entities.Observation{
				{
					Plugin: "aws",
					Config: map[string]interface{}{
						"service":   "ec2",
						"operation": "describe_security_groups",
						"region":    g.region,
						"filters": []map[string]interface{}{
							{"name": "group-id", "values": []string{sgID}},
						},
					},
					Expect: []string{
						`all(data.security_groups, {!contains(.ingress_rules, {.from_port == 22 && .to_port == 22 && .cidr_blocks contains "0.0.0.0/0"})})`,
					},
				},
			},
		})
	}

	// Generate S3 controls
	for _, bucket := range resources.S3.Buckets {
		// Encryption check
		profile.Controls.Items = append(profile.Controls.Items, entities.Control{
			ID:          fmt.Sprintf("aws-s3-%s-encryption", bucket),
			Name:        fmt.Sprintf("S3 Bucket %s Encryption", bucket),
			Description: "Ensure bucket has encryption enabled",
			Observations: []entities.Observation{
				{
					Plugin: "aws",
					Config: map[string]interface{}{
						"service":   "s3",
						"operation": "get_bucket_encryption",
						"region":    g.region,
						"filters": []map[string]interface{}{
							{"name": "bucket", "values": []string{bucket}},
						},
					},
					Expect: []string{
						"len(data.encryption_rules) > 0",
					},
				},
			},
		})

		// Public Access Block check
		profile.Controls.Items = append(profile.Controls.Items, entities.Control{
			ID:          fmt.Sprintf("aws-s3-%s-public-access-block", bucket),
			Name:        fmt.Sprintf("S3 Bucket %s Public Access Block", bucket),
			Description: "Ensure bucket has public access block enabled",
			Observations: []entities.Observation{
				{
					Plugin: "aws",
					Config: map[string]interface{}{
						"service":   "s3",
						"operation": "get_public_access_block",
						"region":    g.region,
						"filters": []map[string]interface{}{
							{"name": "bucket", "values": []string{bucket}},
						},
					},
					Expect: []string{
						"data.public_access_block.block_public_acls == true",
						"data.public_access_block.block_public_policy == true",
					},
				},
			},
		})
	}

	// Generate IAM controls
	if len(resources.IAM.Users) > 0 {
		profile.Controls.Items = append(profile.Controls.Items, entities.Control{
			ID:          "aws-iam-password-policy",
			Name:        "AWS IAM Password Policy",
			Description: "Ensure account has a strong password policy",
			Observations: []entities.Observation{
				{
					Plugin: "aws",
					Config: map[string]interface{}{
						"service":   "iam",
						"operation": "get_account_password_policy",
					},
					Expect: []string{
						"data.password_policy.minimum_password_length >= 14",
						"data.password_policy.require_symbols == true",
						"data.password_policy.require_numbers == true",
					},
				},
			},
		})
	}

	// Generate VPC controls
	for _, vpcID := range resources.VPC.VPCs {
		profile.Controls.Items = append(profile.Controls.Items, entities.Control{
			ID:          fmt.Sprintf("aws-vpc-%s-default", vpcID),
			Name:        fmt.Sprintf("VPC %s Default Config", vpcID),
			Description: "Ensure VPC is not the default VPC",
			Observations: []entities.Observation{
				{
					Plugin: "aws",
					Config: map[string]interface{}{
						"service":   "vpc",
						"operation": "describe_vpcs",
						"region":    g.region,
						"filters": []map[string]interface{}{
							{"name": "vpc-id", "values": []string{vpcID}},
						},
					},
					Expect: []string{
						"all(data.vpcs, {.is_default == false})",
					},
				},
			},
		})
	}

	return profile, nil
}
