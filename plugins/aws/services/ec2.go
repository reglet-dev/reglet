// Package services implements AWS service-specific compliance checks.
package services

import (
	"context"
	"encoding/xml"
	"fmt"

	"github.com/reglet-dev/reglet-sdk/go/application/plugin"
	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
	"github.com/reglet-dev/reglet/plugins/aws/core"
)

// EC2Service handles EC2 compliance checks.
type EC2Service struct {
	plugin.Service `name:"ec2" desc:"EC2 compute instance security checks"`

	DescribeSecurityGroups    plugin.Op `desc:"Find security groups with open SSH/RDP to 0.0.0.0/0" method:"DescribeSecurityGroupsHandler"`
	DescribeInstancesMetadata plugin.Op `desc:"Verify IMDSv2 enforcement on EC2 instances" method:"DescribeInstancesMetadataHandler"`
}

// Auto-register on package import
func init() {
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

// SecurityGroupInfo is the evidence format for a security group.
type SecurityGroupInfo struct {
	Tags         map[string]string `json:"tags"`
	GroupID      string            `json:"group_id"`
	GroupName    string            `json:"group_name"`
	VpcID        string            `json:"vpc_id"`
	Description  string            `json:"description"`
	IngressRules []IngressRule     `json:"ingress_rules"`
}

type IngressRule struct {
	Description    string   `json:"description,omitempty"`
	Protocol       string   `json:"protocol"`
	CidrBlocks     []string `json:"cidr_blocks"`
	Ipv6CidrBlocks []string `json:"ipv6_cidr_blocks,omitempty"`
	FromPort       int      `json:"from_port"`
	ToPort         int      `json:"to_port"`
}

func (s *EC2Service) DescribeSecurityGroupsHandler(ctx context.Context, req *plugin.Request) (*entities.Result, error) {
	client := req.Client.(*core.AWSClient)
	cfg := req.Config.(*core.AWSConfig)

	// Build parameters
	params := make(map[string]string)

	// Add filters if provided
	filterIdx := 1
	for name, values := range cfg.Filters {
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
		return entities.ResultErrorPtr("internal", fmt.Sprintf("Failed to parse AWS response: %v", err)), nil
	}

	return processSecurityGroups(client.Creds.Region, &resp)
}

func processSecurityGroups(region string, resp *DescribeSecurityGroupsResponse) (*entities.Result, error) {
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

	data := map[string]any{
		"service":         "ec2",
		"operation":       "describe_security_groups",
		"region":          region,
		"security_groups": securityGroups,
		"total_groups":    len(securityGroups),
		"open_ssh_groups": openSSHGroups,
		"open_rdp_groups": openRDPGroups,
	}

	if len(openSSHGroups) == 0 && len(openRDPGroups) == 0 {
		return entities.ResultSuccessPtr("No security groups with open SSH or RDP", data), nil
	}

	msg := fmt.Sprintf("%d security group(s) with open SSH, %d with open RDP",
		len(openSSHGroups), len(openRDPGroups))
	return entities.ResultFailurePtr(msg, data), nil
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

// InstanceMetadataInfo is the evidence format for instance metadata settings.
type InstanceMetadataInfo struct {
	Tags            map[string]string `json:"tags"`
	InstanceID      string            `json:"instance_id"`
	InstanceType    string            `json:"instance_type"`
	State           string            `json:"state"`
	MetadataOptions MetadataOptions   `json:"metadata_options"`
	IMDSv2Enforced  bool              `json:"imdsv2_enforced"`
}

type MetadataOptions struct {
	HTTPTokens   string `json:"http_tokens"`
	HTTPEndpoint string `json:"http_endpoint"`
}

func (s *EC2Service) DescribeInstancesMetadataHandler(ctx context.Context, req *plugin.Request) (*entities.Result, error) {
	client := req.Client.(*core.AWSClient)
	cfg := req.Config.(*core.AWSConfig)

	// Build parameters - filter for running instances
	params := map[string]string{
		"Filter.1.Name":    "instance-state-name",
		"Filter.1.Value.1": "running",
	}

	// Add additional filters if provided
	filterIdx := 2
	for name, values := range cfg.Filters {
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
		return entities.ResultErrorPtr("internal", fmt.Sprintf("Failed to parse AWS response: %v", err)), nil
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

	data := map[string]any{
		"service":                 "ec2",
		"operation":               "describe_instances_metadata",
		"region":                  client.Creds.Region,
		"instances":               instances,
		"total_instances":         len(instances),
		"non_compliant_instances": nonCompliantInstances,
	}

	if len(nonCompliantInstances) == 0 {
		return entities.ResultSuccessPtr("All instances enforce IMDSv2", data), nil
	}

	msg := fmt.Sprintf("%d instance(s) do not enforce IMDSv2", len(nonCompliantInstances))
	return entities.ResultFailurePtr(msg, data), nil
}
