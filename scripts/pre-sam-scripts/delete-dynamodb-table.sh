#!/usr/bin/env bash

if [[ -z "${AWS_PROFILE}" ]]; then
  echo "AWS_PROFILE must be set"
  exit 1
fi

aws dynamodb delete-table --table-name mike-price-test-recordings-2
