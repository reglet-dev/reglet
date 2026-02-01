package services

import (
	"context"
	"net/http"
	"testing"

	"github.com/reglet-dev/reglet-sdk/go/application/plugin"
	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
	"github.com/reglet-dev/reglet-sdk/go/domain/ports"
	"github.com/reglet-dev/reglet/plugins/aws/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleDescribeSecurityGroups_Secure(t *testing.T) {
	mockXML := `<?xml version="1.0" encoding="UTF-8"?>
    <DescribeSecurityGroupsResponse>
        <securityGroupInfo>
            <item>
                <groupId>sg-123</groupId>
                <groupName>secure-group</groupName>
                <ipPermissions>
                    <item>
                        <ipProtocol>tcp</ipProtocol>
                        <fromPort>443</fromPort>
                        <toPort>443</toPort>
                        <ipRanges>
                            <item><cidrIp>0.0.0.0/0</cidrIp></item>
                        </ipRanges>
                    </item>
                </ipPermissions>
            </item>
        </securityGroupInfo>
    </DescribeSecurityGroupsResponse>`

	mockClient := &MockHTTPClient{
		Response: &ports.HTTPResponse{
			StatusCode: http.StatusOK,
			Body:       []byte(mockXML),
		},
	}

	creds := &core.AWSCredentials{Region: "us-east-1"}
	client := core.NewAWSClient(creds, 30)
	client.HTTPClient = mockClient

	cfg := &core.AWSConfig{Service: "ec2", Operation: "describe_security_groups"}

	svc := &EC2Service{}
	req := &plugin.Request{
		Client: client,
		Config: cfg,
	}

	result, err := svc.DescribeSecurityGroupsHandler(context.Background(), req)
	require.NoError(t, err)

	assert.Equal(t, entities.ResultStatusSuccess, result.Status)
	assert.Equal(t, 0, len(result.Data["open_ssh_groups"].([]SecurityGroupInfo)))
}

func TestHandleDescribeSecurityGroups_OpenSSH(t *testing.T) {
	mockXML := `<?xml version="1.0" encoding="UTF-8"?>
    <DescribeSecurityGroupsResponse>
        <securityGroupInfo>
            <item>
                <groupId>sg-bad</groupId>
                <groupName>insecure-group</groupName>
                <ipPermissions>
                    <item>
                        <ipProtocol>tcp</ipProtocol>
                        <fromPort>22</fromPort>
                        <toPort>22</toPort>
                        <ipRanges>
                            <item><cidrIp>0.0.0.0/0</cidrIp></item>
                        </ipRanges>
                    </item>
                </ipPermissions>
            </item>
        </securityGroupInfo>
    </DescribeSecurityGroupsResponse>`

	mockClient := &MockHTTPClient{
		Response: &ports.HTTPResponse{
			StatusCode: http.StatusOK,
			Body:       []byte(mockXML),
		},
	}

	creds := &core.AWSCredentials{Region: "us-east-1"}
	client := core.NewAWSClient(creds, 30)
	client.HTTPClient = mockClient

	cfg := &core.AWSConfig{Service: "ec2", Operation: "describe_security_groups"}

	svc := &EC2Service{}
	req := &plugin.Request{
		Client: client,
		Config: cfg,
	}

	result, err := svc.DescribeSecurityGroupsHandler(context.Background(), req)
	require.NoError(t, err)

	assert.Equal(t, entities.ResultStatusFailure, result.Status)
	assert.Equal(t, 1, len(result.Data["open_ssh_groups"].([]SecurityGroupInfo)))
}

func TestHandleDescribeInstancesMetadata_Compliant(t *testing.T) {
	mockXML := `<?xml version="1.0" encoding="UTF-8"?>
    <DescribeInstancesResponse>
        <reservationSet>
            <item>
                <instancesSet>
                    <item>
                        <instanceId>i-123</instanceId>
                        <instanceState><name>running</name></instanceState>
                        <metadataOptions>
                            <httpTokens>required</httpTokens>
                        </metadataOptions>
                    </item>
                </instancesSet>
            </item>
        </reservationSet>
    </DescribeInstancesResponse>`

	mockClient := &MockHTTPClient{
		Response: &ports.HTTPResponse{
			StatusCode: http.StatusOK,
			Body:       []byte(mockXML),
		},
	}

	creds := &core.AWSCredentials{Region: "us-east-1"}
	client := core.NewAWSClient(creds, 30)
	client.HTTPClient = mockClient

	cfg := &core.AWSConfig{Service: "ec2", Operation: "describe_instances_metadata"}

	svc := &EC2Service{}
	req := &plugin.Request{
		Client: client,
		Config: cfg,
	}

	result, err := svc.DescribeInstancesMetadataHandler(context.Background(), req)
	require.NoError(t, err)

	assert.Equal(t, entities.ResultStatusSuccess, result.Status)
	assert.Equal(t, 0, len(result.Data["non_compliant_instances"].([]InstanceMetadataInfo)))
}

func TestHandleDescribeInstancesMetadata_NonCompliant(t *testing.T) {
	mockXML := `<?xml version="1.0" encoding="UTF-8"?>
    <DescribeInstancesResponse>
        <reservationSet>
            <item>
                <instancesSet>
                    <item>
                        <instanceId>i-bad</instanceId>
                        <instanceState><name>running</name></instanceState>
                        <metadataOptions>
                            <httpTokens>optional</httpTokens>
                        </metadataOptions>
                    </item>
                </instancesSet>
            </item>
        </reservationSet>
    </DescribeInstancesResponse>`

	mockClient := &MockHTTPClient{
		Response: &ports.HTTPResponse{
			StatusCode: http.StatusOK,
			Body:       []byte(mockXML),
		},
	}

	creds := &core.AWSCredentials{Region: "us-east-1"}
	client := core.NewAWSClient(creds, 30)
	client.HTTPClient = mockClient

	cfg := &core.AWSConfig{Service: "ec2", Operation: "describe_instances_metadata"}

	svc := &EC2Service{}
	req := &plugin.Request{
		Client: client,
		Config: cfg,
	}

	result, err := svc.DescribeInstancesMetadataHandler(context.Background(), req)
	require.NoError(t, err)

	assert.Equal(t, entities.ResultStatusFailure, result.Status)
	assert.Equal(t, 1, len(result.Data["non_compliant_instances"].([]InstanceMetadataInfo)))
}
