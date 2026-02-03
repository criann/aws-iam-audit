# AWS IAM Audit

[![Go Version](https://img.shields.io/badge/Go-1.25.6-blue)](https://golang.org/)
[![AWS SDK](https://img.shields.io/badge/AWS%20SDK-v2-green)](https://aws.github.io/aws-sdk-go-v2/)

**[Français](README.FR.md)**

A Go-based CLI tool for auditing AWS IAM and S3 resources across your AWS accounts. It retrieves users, groups, roles, their attached policies, and S3 bucket configurations, exporting everything as structured JSON.

## Features

- **IAM Resources Audit**: Users, Groups, and Roles with their attached policies
- **S3 Buckets Audit**: Buckets with policies, versioning, object lock, and public access settings
- **AWS Organizations Support**: Automatically retrieves all accounts in your organization
- **Policy Document Retrieval**: Fetches and decodes policy documents for customer-managed policies
- **Single Profile Operation**: Works with a single AWS profile, automatically handling multi-account access
- **JSON Output**: Clean, structured JSON output for easy parsing and analysis

## Requirements

- Go 1.23 or later
- AWS credentials configured with appropriate permissions (IAM, S3, Organizations)

## Installation

```bash
# Clone the repository
git clone <repository-url>
cd aws-iam-audit

# Download dependencies
go mod download

# Build the binary
go build -o aws-iam-audit

# (Optional) Install to PATH
go install
```

## Usage

```bash
./aws-iam-audit --profile <aws-profile>
```

### Required Flags

- `--profile`: AWS profile to use from `~/.aws/credentials` or `~/.aws/config`

### Example

```bash
./aws-iam-audit --profile my-aws-profile > audit-output.json
```

## Output Format

The tool outputs a JSON structure with the following format:

```json
{
  "metadata": {
    "generated_at": "2026-02-03T13:34:15Z",
    "profile": "my-profile",
    "current_account": "123456789012",
    "current_arn": "arn:aws:iam::123456789012:user/..."
  },
  "accounts": [
    {
      "account_id": "123456789012",
      "account_name": "production",
      "account_status": "ACTIVE",
      "resources": {
        "users": [...],
        "groups": [...],
        "roles": [...],
        "buckets": [...]
      }
    }
  ]
}
```

## Required AWS Permissions

The AWS profile used must have the following permissions:

- `iam:ListUsers`, `iam:GetUser`
- `iam:ListGroups`, `iam:GetGroup`
- `iam:ListRoles`, `iam:GetRole`
- `iam:ListAttachedUserPolicies`, `iam:GetPolicy`, `iam:GetPolicyVersion`
- `iam:ListAttachedGroupPolicies`
- `iam:ListAttachedRolePolicies`
- `s3:ListAllMyBuckets`, `s3:GetBucketLocation`, `s3:GetBucketPolicy`
- `s3:GetBucketVersioning`, `s3:GetObjectLockConfiguration`, `s3:GetPublicAccessBlock`
- `organizations:ListAccounts` (optional, for multi-account support)
- `sts:GetCallerIdentity`

## Example Output

See [example.json](example.json) for a complete sample output.

## Development

### Run directly

```bash
go run main.go --profile my-aws-profile
```

### Build

```bash
go build -o aws-iam-audit
```

### Run tests

```bash
go test ./...
```

## License

MIT
