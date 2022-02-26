#!/usr/bin/env bash

if [[ -z "${AWS_PROFILE}" ]]; then
  echo "AWS_PROFILE must be set"
  exit 1
fi

# force remove contents (except versioned objects)
aws s3 rb "s3://mike-price-test-recordings-image-upload" --force