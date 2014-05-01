#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

go test *.go
