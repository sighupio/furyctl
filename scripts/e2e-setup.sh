#!/bin/bash
# Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# E2E Test Setup Script
# Sets up SSH keys, netrc authentication, and AWS resources needed for e2e tests

set -euo pipefail

echo "🚀 Starting E2E test environment setup..."

# Setup SSH key for Git operations
setup_ssh_key() {
    echo "🔑 Setting up SSH key..."
    
    if [ -z "${GITHUB_SSH:-}" ]; then
        echo "❌ Error: GITHUB_SSH environment variable not set"
        exit 1
    fi
    
    # Create SSH directory with correct permissions
    mkdir -p /root/.ssh
    chmod 700 /root/.ssh
    
    # Setup SSH key (fix the tr/sed issues from inline commands)
    echo "${GITHUB_SSH}" | tr -d '\r' > /root/.ssh/id_rsa
    chmod 600 /root/.ssh/id_rsa
    
    # Create and configure known_hosts
    touch /root/.ssh/known_hosts
    chmod 600 /root/.ssh/known_hosts
    
    echo "✅ SSH key setup complete"
}

# Setup netrc file for private repository authentication
setup_netrc() {
    echo "📝 Setting up netrc authentication..."
    
    if [ -z "${NETRC_FILE:-}" ]; then
        echo "⚠️  Warning: NETRC_FILE environment variable not set, skipping netrc setup"
        return 0
    fi
    
    echo "${NETRC_FILE}" > /root/.netrc
    chmod 600 /root/.netrc
    
    echo "✅ Netrc setup complete"
}

# Configure SSH known hosts
setup_ssh_known_hosts() {
    echo "🌐 Setting up SSH known hosts..."
    
    # Create SSH system directory
    mkdir -p /etc/ssh
    
    # Add GitHub to known hosts (suppress stderr noise)
    ssh-keyscan -H github.com > /etc/ssh/ssh_known_hosts 2> /dev/null || {
        echo "⚠️  Warning: Failed to add github.com to known_hosts"
    }
    
    echo "✅ SSH known hosts setup complete"
}

# Create required S3 bucket for Terraform state storage
setup_s3_bucket() {
    echo "🪣 Setting up S3 bucket for Terraform state..."
    
    # Check required AWS environment variables
    if [ -z "${AWS_ACCESS_KEY_ID:-}" ] || [ -z "${AWS_SECRET_ACCESS_KEY:-}" ] || [ -z "${AWS_DEFAULT_REGION:-}" ]; then
        echo "❌ Error: AWS credentials not properly set (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_DEFAULT_REGION)"
        exit 1
    fi
    
    if [ -z "${TERRAFORM_TF_STATES_BUCKET_NAME:-}" ]; then
        echo "❌ Error: TERRAFORM_TF_STATES_BUCKET_NAME environment variable not set"
        exit 1
    fi
    
    # Create bucket if it doesn't exist in the specified region
    # Using the same logic as the original inline command
    if ! (test ! $(aws s3api get-bucket-location --bucket "${TERRAFORM_TF_STATES_BUCKET_NAME}" --output text --no-cli-pager 2>/dev/null | grep "${AWS_DEFAULT_REGION}")); then
        echo "📦 Bucket ${TERRAFORM_TF_STATES_BUCKET_NAME} already exists in region ${AWS_DEFAULT_REGION}"
    else
        echo "📦 Creating S3 bucket: ${TERRAFORM_TF_STATES_BUCKET_NAME} in region ${AWS_DEFAULT_REGION}"
        aws s3 mb "s3://${TERRAFORM_TF_STATES_BUCKET_NAME}" --region "${AWS_DEFAULT_REGION}"
        echo "✅ S3 bucket created successfully"
    fi
}

# Main execution
main() {
    setup_ssh_key
    setup_netrc
    setup_ssh_known_hosts
    setup_s3_bucket
    
    echo "🎉 E2E test environment setup completed successfully!"
}

# Execute main function
main "$@"