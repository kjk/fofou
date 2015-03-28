#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

gdep go test *.go
