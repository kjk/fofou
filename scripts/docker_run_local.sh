#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

. ./docker_build.bash

docker run --rm -it -v ~/data/fofou:/data -p 5020:80 fofou:latest
