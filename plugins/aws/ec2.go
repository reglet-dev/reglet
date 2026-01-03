//go:build wasip1

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	regletsdk "github.com/whiskeyjimbo/reglet/sdk"
)

func (p *awsPlugin) handleEC2(ctx context.Context, cfg AWSConfig) (regletsdk.Evidence, error) {
	creds, err := getCredentials(cfg)
	if err != nil {
		return regletsdk.Failure("auth", err.Error()), nil
	}

	awsCfg, err := loadAWSConfig(ctx, creds, p.client)
	if err != nil {
		return regletsdk.Failure("internal", fmt.Sprintf("failed to load AWS config: %v", err)), nil
	}

	client := ec2.NewFromConfig(awsCfg)

	switch cfg.Operation {
	case "describe_instances":
		return p.describeInstances(ctx, client, cfg)
	case "describe_security_groups":
		return p.describeSecurityGroups(ctx, client, cfg)
	case "describe_volumes":
		return p.describeVolumes(ctx, client, cfg)
	default:
		return regletsdk.Failure("config", fmt.Sprintf("unsupported EC2 operation: %s", cfg.Operation)), nil
	}
}

func (p *awsPlugin) describeInstances(ctx context.Context, client *ec2.Client, cfg AWSConfig) (regletsdk.Evidence, error) {
	input := &ec2.DescribeInstancesInput{}
	// TODO: Apply filters from cfg.Filters

	start := time.Now()
	output, err := client.DescribeInstances(ctx, input)
	duration := time.Since(start).Milliseconds()

	if err != nil {
		return regletsdk.Failure("network", fmt.Sprintf("DescribeInstances failed: %v", err)), nil
	}

	// Transform output to simplified map for Reglet
	instances := make([]map[string]interface{}, 0)
	for _, reservation := range output.Reservations {
		for _, instance := range reservation.Instances {
			inst := map[string]interface{}{
				"instance_id":      *instance.InstanceId,
				"instance_type":    string(instance.InstanceType),
				"state":            string(instance.State.Name),
				"launch_time":      instance.LaunchTime.Format(time.RFC3339),
				"tags":             convertTags(instance.Tags),
			}
			if instance.Placement != nil && instance.Placement.AvailabilityZone != nil {
				inst["availability_zone"] = *instance.Placement.AvailabilityZone
			}
			if instance.PrivateIpAddress != nil {
				inst["private_ip"] = *instance.PrivateIpAddress
			}
			if instance.PublicIpAddress != nil {
				inst["public_ip"] = *instance.PublicIpAddress
			}
			instances = append(instances, inst)
		}
	}

	return regletsdk.Success(map[string]interface{}{
		"instances":        instances,
		"response_time_ms": duration,
		"region":           cfg.Region,
	}), nil
}

func (p *awsPlugin) describeSecurityGroups(ctx context.Context, client *ec2.Client, cfg AWSConfig) (regletsdk.Evidence, error) {
	input := &ec2.DescribeSecurityGroupsInput{}
	// TODO: Apply filters

	start := time.Now()
	output, err := client.DescribeSecurityGroups(ctx, input)
	duration := time.Since(start).Milliseconds()

	if err != nil {
		return regletsdk.Failure("network", fmt.Sprintf("DescribeSecurityGroups failed: %v", err)), nil
	}

	sgs := make([]map[string]interface{}, 0)
	for _, sg := range output.SecurityGroups {
		sgs = append(sgs, map[string]interface{}{
			"group_id":    *sg.GroupId,
			"group_name":  *sg.GroupName,
			"description": *sg.Description,
			"vpc_id":      *sg.VpcId,
			"tags":        convertTags(sg.Tags),
			"ingress_rules": convertPermissions(sg.IpPermissions),
			"egress_rules":  convertPermissions(sg.IpPermissionsEgress),
		})
	}

	return regletsdk.Success(map[string]interface{}{
		"security_groups":  sgs,
		"response_time_ms": duration,
		"region":           cfg.Region,
	}), nil
}

func (p *awsPlugin) describeVolumes(ctx context.Context, client *ec2.Client, cfg AWSConfig) (regletsdk.Evidence, error) {
	input := &ec2.DescribeVolumesInput{}
	// TODO: Apply filters

	start := time.Now()
	output, err := client.DescribeVolumes(ctx, input)
	duration := time.Since(start).Milliseconds()

	if err != nil {
		return regletsdk.Failure("network", fmt.Sprintf("DescribeVolumes failed: %v", err)), nil
	}

	volumes := make([]map[string]interface{}, 0)
	for _, vol := range output.Volumes {
		v := map[string]interface{}{
			"volume_id":         *vol.VolumeId,
			"size":              *vol.Size,
			"state":             string(vol.State),
			"availability_zone": *vol.AvailabilityZone,
			"encrypted":         *vol.Encrypted,
			"tags":              convertTags(vol.Tags),
		}
		if vol.KmsKeyId != nil {
			v["kms_key_id"] = *vol.KmsKeyId
		}
		volumes = append(volumes, v)
	}

	return regletsdk.Success(map[string]interface{}{
		"volumes":          volumes,
		"response_time_ms": duration,
		"region":           cfg.Region,
	}), nil
}

// Helpers

func convertTags(tags []types.Tag) map[string]string {
	result := make(map[string]string)
	for _, tag := range tags {
		result[*tag.Key] = *tag.Value
	}
	return result
}

func convertPermissions(perms []types.IpPermission) []map[string]interface{} {
	result := make([]map[string]interface{}, 0)
	for _, perm := range perms {
		p := map[string]interface{}{
			"protocol": *perm.IpProtocol,
		}
		if perm.FromPort != nil {
			p["from_port"] = *perm.FromPort
		}
		if perm.ToPort != nil {
			p["to_port"] = *perm.ToPort
		}
		
		ranges := make([]string, 0)
		for _, r := range perm.IpRanges {
			ranges = append(ranges, *r.CidrIp)
		}
		if len(ranges) > 0 {
			p["cidr_blocks"] = ranges
		}
		result = append(result, p)
	}
	return result
}
