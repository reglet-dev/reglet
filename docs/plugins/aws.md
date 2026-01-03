# AWS Plugin

The `aws` plugin provides infrastructure validation and compliance checking for Amazon Web Services.

## Capabilities

The plugin requires the following capabilities:

*   `network`: `outbound:443` (to communicate with AWS APIs)
*   `env`: `AWS_*` (to access credentials and region information)

## Configuration

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `service` | string | Yes | AWS service (ec2, s3, iam, vpc) |
| `operation` | string | Yes | Service operation (e.g., describe_instances) |
| `region` | string | No | AWS region |
| `filters` | array | No | API filters |

## Examples

### EC2: Check for public instances

```yaml
- id: aws-ec2-no-public-instances
  observations:
    - plugin: aws
      config:
        service: ec2
        operation: describe_instances
      expect:
        - "all(data.instances, {not has(., 'public_ip')})"
```

### S3: Check for bucket encryption

```yaml
- id: aws-s3-bucket-encryption
  observations:
    - plugin: aws
      config:
        service: s3
        operation: get_bucket_encryption
        filters:
          - name: bucket
            values: [my-bucket-name]
      expect:
        - "len(data.encryption_rules) > 0"
```

### IAM: Check password policy

```yaml
- id: aws-iam-strong-password-policy
  observations:
    - plugin: aws
      config:
        service: iam
        operation: get_account_password_policy
      expect:
        - "data.password_policy.minimum_password_length >= 14"
        - "data.password_policy.require_symbols == true"
```

### VPC: Check for non-default VPC

```yaml
- id: aws-vpc-not-default
  observations:
    - plugin: aws
      config:
        service: vpc
        operation: describe_vpcs
        filters:
          - name: vpc-id
            values: [vpc-12345678]
      expect:
        - "all(data.vpcs, {.is_default == false})"
```

## Discovery

Use `reglet init aws` to automatically generate a profile by scanning your AWS infrastructure.
