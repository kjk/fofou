#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

GOOS=linux GOARCH=amd64 go build -o fofou_linux

docker build --tag fofou:latest .
