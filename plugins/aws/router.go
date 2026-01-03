//go:build wasip1

package main

import (
	"context"
	"fmt"

	regletsdk "github.com/whiskeyjimbo/reglet/sdk"
)

func (p *awsPlugin) handleService(ctx context.Context, cfg AWSConfig) (regletsdk.Evidence, error) {
	// Route to service handler
	switch cfg.Service {
	case "ec2":
		return p.handleEC2(ctx, cfg)
	case "s3":
		return p.handleS3(ctx, cfg)
	case "iam":
		return p.handleIAM(ctx, cfg)
	case "vpc":
		return p.handleVPC(ctx, cfg)
	default:
		return regletsdk.Failure("config", fmt.Sprintf("unsupported service: %s", cfg.Service)), nil
	}
}
