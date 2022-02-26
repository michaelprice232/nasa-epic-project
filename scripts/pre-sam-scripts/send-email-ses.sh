#!/usr/bin/env bash

if [[ -z "${AWS_PROFILE}" ]]; then
  echo "AWS_PROFILE must be set"
  exit 1
fi

aws ses send-email --from michaelprice232@outlook.com --destination file://scripts/ses/destination.json --message file://scripts/ses/message.json