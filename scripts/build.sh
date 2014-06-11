#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

godep go build -o fofou_app *.go
