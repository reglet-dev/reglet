package main

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type AWSScanner struct {
	cfg aws.Config
}

func NewAWSScanner(cfg aws.Config) *AWSScanner {
	return &AWSScanner{cfg: cfg}
}

type ScanResult struct {
	EC2 EC2Resources
	S3  S3Resources
	IAM IAMResources
	VPC VPCResources
}

type EC2Resources struct {
	Instances      []string
	SecurityGroups []string
}

type S3Resources struct {
	Buckets []string
}

type IAMResources struct {
	Users []string
}

type VPCResources struct {
	VPCs    []string
	Subnets []string
}

func (s *AWSScanner) Scan(ctx context.Context, services []string) (*ScanResult, error) {
	result := &ScanResult{}

	for _, service := range services {
		switch service {
		case "ec2":
			if err := s.scanEC2(ctx, &result.EC2); err != nil {
				return nil, fmt.Errorf("EC2 scan failed: %w", err)
			}
		case "s3":
			if err := s.scanS3(ctx, &result.S3); err != nil {
				return nil, fmt.Errorf("S3 scan failed: %w", err)
			}
		case "iam":
			if err := s.scanIAM(ctx, &result.IAM); err != nil {
				return nil, fmt.Errorf("IAM scan failed: %w", err)
			}
		case "vpc":
			if err := s.scanVPC(ctx, &result.VPC); err != nil {
				return nil, fmt.Errorf("VPC scan failed: %w", err)
			}
		}
	}

	return result, nil
}

func (s *AWSScanner) scanEC2(ctx context.Context, resources *EC2Resources) error {
	client := ec2.NewFromConfig(s.cfg)

	// Scan Instances
	instances, err := client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{})
	if err != nil {
		return err
	}

	for _, r := range instances.Reservations {
		for _, i := range r.Instances {
			resources.Instances = append(resources.Instances, *i.InstanceId)
		}
	}

	// Scan Security Groups
	sgs, err := client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{})
	if err != nil {
		return err
	}

	for _, sg := range sgs.SecurityGroups {
		resources.SecurityGroups = append(resources.SecurityGroups, *sg.GroupId)
	}

	return nil
}

func (s *AWSScanner) scanS3(ctx context.Context, resources *S3Resources) error {
	client := s3.NewFromConfig(s.cfg)

	buckets, err := client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return err
	}

	for _, b := range buckets.Buckets {
		resources.Buckets = append(resources.Buckets, *b.Name)
	}

	return nil
}

func (s *AWSScanner) scanIAM(ctx context.Context, resources *IAMResources) error {
	client := iam.NewFromConfig(s.cfg)

	users, err := client.ListUsers(ctx, &iam.ListUsersInput{})
	if err != nil {
		return err
	}

	for _, u := range users.Users {
		resources.Users = append(resources.Users, *u.UserName)
	}

	return nil
}

func (s *AWSScanner) scanVPC(ctx context.Context, resources *VPCResources) error {
	client := ec2.NewFromConfig(s.cfg)

	vpcs, err := client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{})
	if err != nil {
		return err
	}

	for _, v := range vpcs.Vpcs {
		resources.VPCs = append(resources.VPCs, *v.VpcId)
	}

	subnets, err := client.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{})
	if err != nil {
		return err
	}

	for _, sub := range subnets.Subnets {
		resources.Subnets = append(resources.Subnets, *sub.SubnetId)
	}

	return nil
}
