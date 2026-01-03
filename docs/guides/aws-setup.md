# AWS Setup & Authentication Guide

This guide explains how to configure authentication for the Reglet AWS plugin.

## Overview

Reglet uses the standard AWS SDK for Go (v2) credential chain, but with strict sandboxing. Plugins cannot access your environment variables unless explicitly allowed by the `env` capability.

## Authentication Methods

### 1. Environment Variables (Recommended for local dev)

Set the standard AWS environment variables in your shell:

```bash
export AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
export AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
export AWS_REGION=us-east-1
```

**Capability Requirement:**
The plugin will request the `env:AWS_*` capability. You must grant this when prompted, or configure it in `~/.reglet/config.yaml`:

```yaml
# ~/.reglet/config.yaml
capabilities:
  - kind: env
    pattern: AWS_*
```

### 2. OIDC (Recommended for CI/CD)

For GitHub Actions, GitLab CI, or other CI providers, use OpenID Connect (OIDC) to obtain temporary credentials without long-lived secrets.

**GitHub Actions Example:**

1.  **Configure AWS IAM Role:** Create a role with a Trust Policy allowing your GitHub repository.
2.  **Workflow Step:** Use `aws-actions/configure-aws-credentials` to assume the role.

```yaml
jobs:
  compliance:
    permissions:
      id-token: write  # Required for OIDC
      contents: read
    steps:
      - name: Configure AWS Credentials
        uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: arn:aws:iam::123456789012:role/RegletRole
          aws-region: us-east-1

      - name: Run Reglet
        run: reglet check profile.yaml
```

The `configure-aws-credentials` action sets the `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and `AWS_SESSION_TOKEN` environment variables, which Reglet's AWS plugin will automatically use (via the `env:AWS_*` capability).

### 3. Shared Config/Credentials File (`~/.aws/credentials`)

**Note:** Direct reading of `~/.aws/credentials` inside the WASM plugin is **not currently supported** by the default `env`-based authentication pattern. The WASM sandbox prevents filesystem access unless explicitly mounted.

To use your local AWS profile:

```bash
# Export profile credentials to environment variables
export AWS_PROFILE=my-profile
# Or use a helper like aws-vault
aws-vault exec my-profile -- reglet check profile.yaml
```

## Permissions

The IAM identity used by Reglet needs read-only permissions for the services you are scanning.

**Minimal Policy Example:**

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "ec2:DescribeInstances",
                "ec2:DescribeSecurityGroups",
                "ec2:DescribeVolumes",
                "s3:ListAllMyBuckets",
                "s3:GetBucketEncryption",
                "s3:GetPublicAccessBlock",
                "iam:ListUsers",
                "iam:GetAccountPasswordPolicy"
            ],
            "Resource": "*"
        }
    ]
}
```

## Troubleshooting

**"No credentials found"**
*   Ensure `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` are set.
*   Ensure you granted the `env:AWS_*` capability when prompted.

**"Access Denied"**
*   Check the IAM policy attached to your user or role.
*   Verify you are in the correct AWS region.

**"Capability denied"**
*   If you see `capability env:AWS_ACCESS_KEY_ID denied`, check your `~/.reglet/config.yaml` or run with `--security=permissive` (not recommended for production).
