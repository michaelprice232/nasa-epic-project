#!/usr/bin/env bash

if [[ -z "${AWS_PROFILE}" ]]; then
  echo "AWS_PROFILE must be set"
  exit 1
fi

aws dynamodb create-table --table-name mike-price-test-recordings-2 \
  --attribute-definitions AttributeName=Identifier,AttributeType=S AttributeName=FormattedDateStr,AttributeType=S \
  --key-schema AttributeName=Identifier,KeyType=HASH AttributeName=FormattedDateStr,KeyType=RANGE \
  --billing-mode PAY_PER_REQUEST \
  --tags Key=Owner,Value="Michael Price" Key=Purpose,Value="Testing"