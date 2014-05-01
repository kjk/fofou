#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

go build -o fofou_app *.go
./fofou_app
