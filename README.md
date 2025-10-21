# GitLab Terraform MR Commenter

A Go application that processes Terraform plan JSON output and creates formatted comments on GitLab Merge Requests showing added/changed/removed resources with inline diffs.

## Quick Start

```bash
# Build
go build -o gitlab-terraform-mr-commenter ./cmd/gitlab-terraform-mr-commenter

# Set environment variables
export GITLAB_TOKEN="your-token"
export GITLAB_PROJECT_ID="123"
export GITLAB_MR_ID="456"

# Run
./gitlab-terraform-mr-commenter plan.json
```

## Features

- Parses Terraform plan JSON output
- Categorizes resources into Added, Changed, and Removed sections
- Updates existing comments instead of creating duplicates
- Supports multiple plan files
- Optional output to file/stdout for dry runs
- Works with both GitLab.com and self-hosted instances

## Usage

### Prerequisites

- Go 1.25 or higher
- Terraform plan in JSON format (`terraform show -json plan.out > plan.json`)

### Environment Variables

```bash
GITLAB_TOKEN       # GitLab personal access token (required)
GITLAB_PROJECT_ID  # GitLab project ID (required) 
GITLAB_MR_ID       # GitLab merge request ID (required)
GITLAB_URL         # GitLab instance URL (optional, defaults to https://gitlab.com)
```

### Running

```bash
# Basic usage
./gitlab-terraform-mr-commenter plan.json

# Multiple plans
./gitlab-terraform-mr-commenter plan1.json plan2.json

# Dry run to stdout
./gitlab-terraform-mr-commenter -o - plan.json

# Output to file
./gitlab-terraform-mr-commenter -o output.md plan.json
```

### GitLab Token Permissions

Required scopes: `api`, `read_repository`

## Example Output

```
## Terraform Plan Summary

### ✅ Add
#### `aws_instance.web`
```diff
+instance_type: t3.micro
+ami: ami-12345678
```

### 🔄 Change  
#### `aws_security_group.web`
```diff
-description: ["old security group"]
+description: ["updated security group"]
-ingress: [{"from_port": 80}]
+ingress: [{"from_port": 443}]
```

### ❌ Destroy
#### `aws_s3_bucket.old_bucket`
```diff
-bucket: my-old-bucket
-region: us-west-2
```
```