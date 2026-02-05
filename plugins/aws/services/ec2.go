// Package services implements AWS service-specific compliance checks.
package services

import (
	"context"
	"encoding/xml"
	"fmt"

	"github.com/reglet-dev/reglet-sdk/application/plugin"
	"github.com/reglet-dev/reglet/plugins/aws/core"
)

// EC2Service handles EC2 compliance checks.
type EC2Service struct {
	plugin.Service `name:"ec2" desc:"EC2 compute instance security checks"`

	DescribeSecurityGroups    plugin.Op[DescribeSecurityGroupsInput, DescribeSecurityGroupsOutput]       `desc:"Find security groups with open SSH/RDP to 0.0.0.0/0" method:"DescribeSecurityGroupsHandler"`
	DescribeInstancesMetadata plugin.Op[DescribeInstancesMetadataInput, DescribeInstancesMetadataOutput] `desc:"Verify IMDSv2 enforcement on EC2 instances" method:"DescribeInstancesMetadataHandler"`
}

// Auto-register on package import
func init() {
	plugin.RegisterOp[DescribeSecurityGroupsInput, DescribeSecurityGroupsOutput]("DescribeSecurityGroups",
		plugin.Example[DescribeSecurityGroupsInput, DescribeSecurityGroupsOutput]{
			Name:        "open_ssh_check",
			Description: "Check for security groups causing open SSH",
			Input:       DescribeSecurityGroupsInput{},
		},
	)

	plugin.RegisterOp[DescribeInstancesMetadataInput, DescribeInstancesMetadataOutput]("DescribeInstancesMetadata",
		plugin.Example[DescribeInstancesMetadataInput, DescribeInstancesMetadataOutput]{
			Name:        "imdsv2_check",
			Description: "Verify IMDSv2 is enforced on running instances",
			Input:       DescribeInstancesMetadataInput{},
		},
	)

	plugin.MustRegisterService(core.Plugin, &EC2Service{})
}

// =============================================================================
// Check 1: Security Groups - No Open SSH/RDP
// =============================================================================

// DescribeSecurityGroupsResponse represents the AWS response.
type DescribeSecurityGroupsResponse struct {
	XMLName           xml.Name `xml:"DescribeSecurityGroupsResponse"`
	SecurityGroupInfo struct {
		Item []SecurityGroupXML `xml:"item"`
	} `xml:"securityGroupInfo"`
}

type SecurityGroupXML struct {
	GroupID       string `xml:"groupId"`
	GroupName     string `xml:"groupName"`
	VpcID         string `xml:"vpcId"`
	Description   string `xml:"groupDescription"`
	IPPermissions struct {
		Item []IPPermissionXML `xml:"item"`
	} `xml:"ipPermissions"`
	Tags struct {
		Item []TagXML `xml:"item"`
	} `xml:"tagSet"`
}

type IPPermissionXML struct {
	IPProtocol string `xml:"ipProtocol"`
	IPRanges   struct {
		Item []struct {
			CidrIP      string `xml:"cidrIp"`
			Description string `xml:"description"`
		} `xml:"item"`
	} `xml:"ipRanges"`
	Ipv6Ranges struct {
		Item []struct {
			CidrIpv6    string `xml:"cidrIpv6"`
			Description string `xml:"description"`
		} `xml:"item"`
	} `xml:"ipv6Ranges"`
	FromPort int `xml:"fromPort"`
	ToPort   int `xml:"toPort"`
}

type TagXML struct {
	Key   string `xml:"key"`
	Value string `xml:"value"`
}

func (s *EC2Service) DescribeSecurityGroupsHandler(ctx context.Context, in *DescribeSecurityGroupsInput) (*DescribeSecurityGroupsOutput, error) {
	client := plugin.GetClient[*core.AWSClient](ctx)

	// Build parameters
	params := make(map[string]string)

	// Add filters if provided
	filterIdx := 1
	for name, values := range in.Filters {
		params[fmt.Sprintf("Filter.%d.Name", filterIdx)] = name
		for i, v := range values {
			params[fmt.Sprintf("Filter.%d.Value.%d", filterIdx, i+1)] = v
		}
		filterIdx++
	}

	// Call AWS API
	body, err := client.Call(ctx, "ec2", "DescribeSecurityGroups", params)
	if err != nil {
		return nil, err
	}

	// Parse response
	var resp DescribeSecurityGroupsResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse AWS response: %w", err)
	}

	// Region override?
	region := client.Creds.Region
	if in.Region != "" {
		region = in.Region
		// TODO: Actually switch client region if needed, but for now reporting
	}

	return processSecurityGroups(region, &resp)
}

func processSecurityGroups(region string, resp *DescribeSecurityGroupsResponse) (*DescribeSecurityGroupsOutput, error) {
	var securityGroups []SecurityGroupInfo
	var openSSHGroups []SecurityGroupInfo
	var openRDPGroups []SecurityGroupInfo

	for _, sg := range resp.SecurityGroupInfo.Item {
		sgInfo := SecurityGroupInfo{
			GroupID:     sg.GroupID,
			GroupName:   sg.GroupName,
			VpcID:       sg.VpcID,
			Description: sg.Description,
			Tags:        make(map[string]string),
		}

		// Convert tags
		for _, tag := range sg.Tags.Item {
			sgInfo.Tags[tag.Key] = tag.Value
		}

		// Check ingress rules
		hasOpenSSH, hasOpenRDP := processIngressRules(&sgInfo, &sg)

		securityGroups = append(securityGroups, sgInfo)
		if hasOpenSSH {
			openSSHGroups = append(openSSHGroups, sgInfo)
		}
		if hasOpenRDP {
			openRDPGroups = append(openRDPGroups, sgInfo)
		}
	}

	return &DescribeSecurityGroupsOutput{
		Region:         region,
		TotalGroups:    len(securityGroups),
		SecurityGroups: securityGroups,
		OpenSSHGroups:  openSSHGroups,
		OpenRDPGroups:  openRDPGroups,
	}, nil
}

func processIngressRules(sgInfo *SecurityGroupInfo, sg *SecurityGroupXML) (bool, bool) {
	hasOpenSSH := false
	hasOpenRDP := false

	for _, perm := range sg.IPPermissions.Item {
		rule := IngressRule{
			Protocol: perm.IPProtocol,
			FromPort: perm.FromPort,
			ToPort:   perm.ToPort,
		}

		for _, ipRange := range perm.IPRanges.Item {
			rule.CidrBlocks = append(rule.CidrBlocks, ipRange.CidrIP)
			if ipRange.CidrIP == "0.0.0.0/0" {
				if isPortOpen(perm.FromPort, perm.ToPort, perm.IPProtocol, 22) {
					hasOpenSSH = true
				}
				if isPortOpen(perm.FromPort, perm.ToPort, perm.IPProtocol, 3389) {
					hasOpenRDP = true
				}
			}
		}

		for _, ipv6Range := range perm.Ipv6Ranges.Item {
			rule.Ipv6CidrBlocks = append(rule.Ipv6CidrBlocks, ipv6Range.CidrIpv6)
			if ipv6Range.CidrIpv6 == "::/0" {
				if isPortOpen(perm.FromPort, perm.ToPort, perm.IPProtocol, 22) {
					hasOpenSSH = true
				}
				if isPortOpen(perm.FromPort, perm.ToPort, perm.IPProtocol, 3389) {
					hasOpenRDP = true
				}
			}
		}

		sgInfo.IngressRules = append(sgInfo.IngressRules, rule)
	}

	return hasOpenSSH, hasOpenRDP
}

func isPortOpen(fromPort, toPort int, protocol string, targetPort int) bool {
	// Protocol -1 means all protocols (so all ports)
	if protocol == "-1" {
		return true
	}
	// Only check TCP
	if protocol != "tcp" {
		return false
	}
	return fromPort <= targetPort && toPort >= targetPort
}

// =============================================================================
// Check 2: IMDSv2 Enforcement
// =============================================================================

// DescribeInstancesResponse represents the AWS response.
type DescribeInstancesResponse struct {
	XMLName        xml.Name `xml:"DescribeInstancesResponse"`
	ReservationSet struct {
		Item []struct {
			InstancesSet struct {
				Item []InstanceXML `xml:"item"`
			} `xml:"instancesSet"`
		} `xml:"item"`
	} `xml:"reservationSet"`
}

type InstanceXML struct {
	InstanceID    string `xml:"instanceId"`
	InstanceType  string `xml:"instanceType"`
	InstanceState struct {
		Name string `xml:"name"`
	} `xml:"instanceState"`
	MetadataOptions struct {
		HTTPTokens   string `xml:"httpTokens"`
		HTTPEndpoint string `xml:"httpEndpoint"`
	} `xml:"metadataOptions"`
	Tags struct {
		Item []TagXML `xml:"item"`
	} `xml:"tagSet"`
}

func (s *EC2Service) DescribeInstancesMetadataHandler(ctx context.Context, in *DescribeInstancesMetadataInput) (*DescribeInstancesMetadataOutput, error) {
	client := plugin.GetClient[*core.AWSClient](ctx)

	// Build parameters - filter for running instances
	params := map[string]string{
		"Filter.1.Name":    "instance-state-name",
		"Filter.1.Value.1": "running",
	}

	// Add additional filters if provided
	filterIdx := 2
	for name, values := range in.Filters {
		params[fmt.Sprintf("Filter.%d.Name", filterIdx)] = name
		for i, v := range values {
			params[fmt.Sprintf("Filter.%d.Value.%d", filterIdx, i+1)] = v
		}
		filterIdx++
	}

	// Call AWS API
	body, err := client.Call(ctx, "ec2", "DescribeInstances", params)
	if err != nil {
		return nil, err
	}

	// Parse response
	var resp DescribeInstancesResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse AWS response: %w", err)
	}

	// Process instances
	var instances []InstanceMetadataInfo
	var nonCompliantInstances []InstanceMetadataInfo

	for _, reservation := range resp.ReservationSet.Item {
		for _, inst := range reservation.InstancesSet.Item {
			info := InstanceMetadataInfo{
				InstanceID:   inst.InstanceID,
				InstanceType: inst.InstanceType,
				State:        inst.InstanceState.Name,
				MetadataOptions: MetadataOptions{
					HTTPTokens:   inst.MetadataOptions.HTTPTokens,
					HTTPEndpoint: inst.MetadataOptions.HTTPEndpoint,
				},
				Tags: make(map[string]string),
			}

			// Convert tags
			for _, tag := range inst.Tags.Item {
				info.Tags[tag.Key] = tag.Value
			}

			// Check IMDSv2 enforcement
			info.IMDSv2Enforced = inst.MetadataOptions.HTTPTokens == "required"

			instances = append(instances, info)
			if !info.IMDSv2Enforced {
				nonCompliantInstances = append(nonCompliantInstances, info)
			}
		}
	}

	return &DescribeInstancesMetadataOutput{
		Region:                client.Creds.Region,
		TotalInstances:        len(instances),
		Instances:             instances,
		NonCompliantInstances: nonCompliantInstances,
	}, nil
}
