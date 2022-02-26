#!/usr/bin/env bash

if [[ -z "${AWS_PROFILE}" ]]; then
  echo "AWS_PROFILE must be set"
  exit 1
fi

BUCKET_NAME="mike-price-test-recordings-image-upload"
BUCKET_URI="s3://${BUCKET_NAME}"

# create bucket
aws s3 mb "${BUCKET_URI}" --region eu-west-1

# enable static website hosting
aws s3 website "${BUCKET_URI}" --index-document "index.html"

# add bucket policy to enable anonymous access
aws s3api put-bucket-policy --bucket "${BUCKET_NAME}" --policy file://scripts/bucket-policy.json