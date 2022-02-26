#!/usr/bin/env bash

# Build the app, copy the external files in and then deploy to AWS via Cloudformation

sam build &&
mkdir .aws-sam/build/MyFunction/images &&
mkdir .aws-sam/build/MyFunction/internal &&
cp -R images/ .aws-sam/build/MyFunction/images/ &&
cp -R internal/ .aws-sam/build/MyFunction/internal/ &&
sam deploy --no-confirm-changeset