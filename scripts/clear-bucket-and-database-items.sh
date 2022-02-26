#!/usr/bin/env bash

# Delete ALL messages and items from the target DynamoDB table and S3 bucket. Used during local testing

if [[ -z "${AWS_PROFILE}" ]]; then
  echo "AWS_PROFILE must be set"
  exit 1
fi

dbTableName="mike-price-test-recordings-2"
uploadS3BucketName="s3://mike-price-test-recordings-image-upload"

aws s3 rm "${uploadS3BucketName}" --recursive

aws dynamodb scan \
  --attributes-to-get Identifier FormattedDateStr \
  --table-name ${dbTableName} --query "Items[*]" \
  | jq --compact-output '.[]' \
  | tr '\n' '\0' \
  | xargs -0 -t -I keyItem \
    aws dynamodb delete-item --table-name ${dbTableName} --key=keyItem