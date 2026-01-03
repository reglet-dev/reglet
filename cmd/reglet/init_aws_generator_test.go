package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProfileGenerator_Generate(t *testing.T) {
	resources := &ScanResult{
		EC2: EC2Resources{
			Instances:      []string{"i-12345678"},
			SecurityGroups: []string{"sg-abcdef"},
		},
		S3: S3Resources{
			Buckets: []string{"test-bucket"},
		},
		IAM: IAMResources{
			Users: []string{"test-user"},
		},
		VPC: VPCResources{
			VPCs: []string{"vpc-123"},
		},
	}

	generator := NewProfileGenerator("cis", "us-east-1")
	profile, err := generator.Generate(resources)

	require.NoError(t, err)
	assert.Equal(t, "aws-generated-profile", profile.Metadata.Name)
	assert.Equal(t, "1.0.0", profile.Metadata.Version)

	// Check if controls were generated
	// Should have: 1 SG control + 2 S3 controls + 1 IAM control + 1 VPC control = 5 controls
	assert.Len(t, profile.Controls.Items, 5)

	// Verify a specific control
	foundS3 := false
	for _, ctrl := range profile.Controls.Items {
		if ctrl.ID == "aws-s3-test-bucket-encryption" {
			foundS3 = true
			assert.Equal(t, "S3 Bucket test-bucket Encryption", ctrl.Name)
			assert.Equal(t, "aws", ctrl.Observations[0].Plugin)
			assert.Equal(t, "s3", ctrl.Observations[0].Config["service"])
			assert.Equal(t, "get_bucket_encryption", ctrl.Observations[0].Config["operation"])
		}
	}
	assert.True(t, foundS3, "Expected S3 control not found")
}
