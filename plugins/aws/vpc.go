//go:build wasip1

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	regletsdk "github.com/whiskeyjimbo/reglet/sdk"
)

func (p *awsPlugin) handleVPC(ctx context.Context, cfg AWSConfig) (regletsdk.Evidence, error) {
	creds, err := getCredentials(cfg)
	if err != nil {
		return regletsdk.Failure("auth", err.Error()), nil
	}

	awsCfg, err := loadAWSConfig(ctx, creds, p.client)
	if err != nil {
		return regletsdk.Failure("internal", fmt.Sprintf("failed to load AWS config: %v", err)), nil
	}

	// VPC operations are actually in the EC2 client in the AWS SDK
	client := ec2.NewFromConfig(awsCfg)

	switch cfg.Operation {
	case "describe_vpcs":
		return p.describeVPCs(ctx, client, cfg)
	case "describe_subnets":
		return p.describeSubnets(ctx, client, cfg)
	case "describe_flow_logs":
		return p.describeFlowLogs(ctx, client, cfg)
	default:
		return regletsdk.Failure("config", fmt.Sprintf("unsupported VPC operation: %s", cfg.Operation)), nil
	}
}

func (p *awsPlugin) describeVPCs(ctx context.Context, client *ec2.Client, cfg AWSConfig) (regletsdk.Evidence, error) {
	input := &ec2.DescribeVpcsInput{}
	// TODO: Apply filters

	start := time.Now()
	output, err := client.DescribeVpcs(ctx, input)
	duration := time.Since(start).Milliseconds()

	if err != nil {
		return regletsdk.Failure("network", fmt.Sprintf("DescribeVpcs failed: %v", err)), nil
	}

	vpcs := make([]map[string]interface{}, 0)
	for _, v := range output.Vpcs {
		vpc := map[string]interface{}{
			"vpc_id":     *v.VpcId,
			"state":      string(v.State),
			"cidr_block": *v.CidrBlock,
			"is_default": *v.IsDefault,
			"tags":       convertTags(v.Tags),
		}
		vpcs = append(vpcs, vpc)
	}

	return regletsdk.Success(map[string]interface{}{
		"vpcs":             vpcs,
		"response_time_ms": duration,
		"region":           cfg.Region,
	}), nil
}

func (p *awsPlugin) describeSubnets(ctx context.Context, client *ec2.Client, cfg AWSConfig) (regletsdk.Evidence, error) {
	input := &ec2.DescribeSubnetsInput{}
	// TODO: Apply filters

	start := time.Now()
	output, err := client.DescribeSubnets(ctx, input)
	duration := time.Since(start).Milliseconds()

	if err != nil {
		return regletsdk.Failure("network", fmt.Sprintf("DescribeSubnets failed: %v", err)), nil
	}

	subnets := make([]map[string]interface{}, 0)
	for _, s := range output.Subnets {
		subnet := map[string]interface{}{
			"subnet_id":         *s.SubnetId,
			"vpc_id":            *s.VpcId,
			"availability_zone": *s.AvailabilityZone,
			"cidr_block":        *s.CidrBlock,
			"tags":              convertTags(s.Tags),
		}
		subnets = append(subnets, subnet)
	}

	return regletsdk.Success(map[string]interface{}{
		"subnets":          subnets,
		"response_time_ms": duration,
		"region":           cfg.Region,
	}), nil
}

func (p *awsPlugin) describeFlowLogs(ctx context.Context, client *ec2.Client, cfg AWSConfig) (regletsdk.Evidence, error) {
	input := &ec2.DescribeFlowLogsInput{}
	// TODO: Apply filters for specific VPC/subnet/ENI

	start := time.Now()
	output, err := client.DescribeFlowLogs(ctx, input)
	duration := time.Since(start).Milliseconds()

	if err != nil {
		return regletsdk.Failure("network", fmt.Sprintf("DescribeFlowLogs failed: %v", err)), nil
	}

	flowLogs := make([]map[string]interface{}, 0)
	for _, fl := range output.FlowLogs {
		flowLog := map[string]interface{}{
			"flow_log_id":   *fl.FlowLogId,
			"resource_id":   *fl.ResourceId,
			"traffic_type":  string(fl.TrafficType),
			"log_status":    *fl.FlowLogStatus,
			"creation_time": fl.CreationTime.Format(time.RFC3339),
		}

		if fl.LogDestinationType != "" {
			flowLog["log_destination_type"] = string(fl.LogDestinationType)
		}
		if fl.LogDestination != nil {
			flowLog["log_destination"] = *fl.LogDestination
		}
		if fl.LogGroupName != nil {
			flowLog["log_group_name"] = *fl.LogGroupName
		}

		flowLog["tags"] = convertTags(fl.Tags)
		flowLogs = append(flowLogs, flowLog)
	}

	return regletsdk.Success(map[string]interface{}{
		"flow_logs":        flowLogs,
		"response_time_ms": duration,
		"region":           cfg.Region,
	}), nil
}
