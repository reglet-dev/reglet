package services

import (
	"context"
	"net/http"
	"testing"

	"github.com/reglet-dev/reglet-sdk/go/application/plugin"
	"github.com/reglet-dev/reglet-sdk/go/domain/ports"
	"github.com/reglet-dev/reglet/plugins/aws/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEC2Examples runs auto-generated tests from registered examples.
func TestEC2Examples(t *testing.T) {
	t.Skip("Requires AWS credentials or mock injection setup for examples")
}

func TestDescribeSecurityGroupsHandler(t *testing.T) {
	// Mock AWS response
	mockResponseXML := `<?xml version="1.0" encoding="UTF-8"?>
    <DescribeSecurityGroupsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
        <requestId>59dbff89-35bd-4eac-99ed-be587EXAMPLE</requestId>
        <securityGroupInfo>
            <item>
                <ownerId>123456789012</ownerId>
                <groupId>sg-903004f8</groupId>
                <groupName>my-security-group</groupName>
                <groupDescription>Enable open SSH access</groupDescription>
                <vpcId>vpc-1a2b3c4d</vpcId>
                <ipPermissions>
                    <item>
                        <ipProtocol>tcp</ipProtocol>
                        <fromPort>22</fromPort>
                        <toPort>22</toPort>
                        <ipRanges>
                            <item>
                                <cidrIp>0.0.0.0/0</cidrIp>
                            </item>
                        </ipRanges>
                        <ipv6Ranges>
                        </ipv6Ranges>
                    </item>
                </ipPermissions>
                <tagSet>
                     <item>
                        <key>Name</key>
                        <value>sg-01</value>
                    </item>
                </tagSet>
            </item>
        </securityGroupInfo>
    </DescribeSecurityGroupsResponse>`

	mockClient := &MockHTTPClient{
		Response: &ports.HTTPResponse{
			StatusCode: http.StatusOK,
			Body:       []byte(mockResponseXML),
		},
	}

	creds := &core.AWSCredentials{AccessKeyID: "test", SecretAccessKey: "test", Region: "us-east-1"}
	awsClient := core.NewAWSClient(creds, 30)
	awsClient.HTTPClient = mockClient

	// Test directly injecting client via context
	ctx := context.Background()
	ctx = plugin.WithClient(ctx, awsClient)

	svc := &EC2Service{}
	input := &DescribeSecurityGroupsInput{
		Filters: map[string][]string{
			"vpc-id": {"vpc-1a2b3c4d"},
		},
	}

	out, err := svc.DescribeSecurityGroupsHandler(ctx, input)
	require.NoError(t, err)

	assert.Equal(t, 1, out.TotalGroups)
	assert.Equal(t, 1, len(out.OpenSSHGroups))
	assert.Equal(t, "sg-903004f8", out.SecurityGroups[0].GroupID)
	assert.Equal(t, "sg-01", out.SecurityGroups[0].Tags["Name"])
}

func TestDescribeInstancesMetadataHandler(t *testing.T) {
	mockResponseXML := `<?xml version="1.0" encoding="UTF-8"?>
    <DescribeInstancesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
        <reservationSet>
            <item>
                <instancesSet>
                    <item>
                        <instanceId>i-1234567890abcdef0</instanceId>
                        <instanceType>t2.micro</instanceType>
                        <instanceState>
                            <code>16</code>
                            <name>running</name>
                        </instanceState>
                        <metadataOptions>
                            <state>applied</state>
                            <httpTokens>optional</httpTokens>
                            <httpEndpoint>enabled</httpEndpoint>
                        </metadataOptions>
                    </item>
                </instancesSet>
            </item>
        </reservationSet>
    </DescribeInstancesResponse>`

	mockClient := &MockHTTPClient{
		Response: &ports.HTTPResponse{
			StatusCode: http.StatusOK,
			Body:       []byte(mockResponseXML),
		},
	}

	creds := &core.AWSCredentials{AccessKeyID: "test", SecretAccessKey: "test", Region: "us-east-1"}
	awsClient := core.NewAWSClient(creds, 30)
	awsClient.HTTPClient = mockClient

	ctx := context.Background()
	ctx = plugin.WithClient(ctx, awsClient)

	svc := &EC2Service{}
	input := &DescribeInstancesMetadataInput{}

	out, err := svc.DescribeInstancesMetadataHandler(ctx, input)
	require.NoError(t, err)

	assert.Equal(t, 1, out.TotalInstances)
	assert.Equal(t, 1, len(out.NonCompliantInstances))
	assert.False(t, out.Instances[0].IMDSv2Enforced)
}
