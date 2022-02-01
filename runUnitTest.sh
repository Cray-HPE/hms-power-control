#!/usr/bin/env bash
#
# MIT License
#
# (C) Copyright [2021-2022] Hewlett Packard Enterprise Development LP
#
# Permission is hereby granted, free of charge, to any person obtaining a
# copy of this software and associated documentation files (the "Software"),
# to deal in the Software without restriction, including without limitation
# the rights to use, copy, modify, merge, publish, distribute, sublicense,
# and/or sell copies of the Software, and to permit persons to whom the
# Software is furnished to do so, subject to the following conditions:
#
# The above copyright notice and this permission notice shall be included
# in all copies or substantial portions of the Software.
#
# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
# FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL
# THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR
# OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE,
# ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR
# OTHER DEALINGS IN THE SOFTWARE.
#

#TODO can I toggle on and off set -e?
set -x
# Setup environment variables
#export GOPATH=$(pwd)/go
RANDY=$(echo $RANDOM | md5sum | awk '{print $1}')


# Parse command line arguments
function usage() {
  echo "$FUNCNAME: $0 [-h] [-k]"
  exit 0
}

while getopts "hk" opt; do
  case $opt in
  h) usage ;;
  *) usage ;;
  esac
done

# Configure docker compose
export COMPOSE_PROJECT_NAME=$RANDY
export COMPOSE_FILE=docker-compose.test.unit.yaml

echo "RANDY: ${RANDY}"
echo "Compose project name: $COMPOSE_PROJECT_NAME"

# It's possible we don't have docker-compose, so if necessary bring our own.
docker_compose_exe=$(command -v docker-compose)
if ! [[ -x "$docker_compose_exe" ]]; then
  if ! [[ -x "./docker-compose" ]]; then
    echo "Getting docker-compose..."
    curl -L "https://github.com/docker/compose/releases/download/1.23.2/docker-compose-$(uname -s)-$(uname -m)" \
      -o ./docker-compose

    if [[ $? -ne 0 ]]; then
      echo "Failed to fetch docker-compose!"
      exit 1
    fi

    chmod +x docker-compose
  fi
  docker_compose_exe="./docker-compose"
fi

function cleanup() {
  ${docker_compose_exe} down
  if ! [[ $? -eq 0 ]]; then
    echo "Failed to decompose environment!"
    exit 1
  fi
  exit $1
}

# Step 3) Get the base containers running
echo "Starting containers..."
${docker_compose_exe} up  -d --build
# TODO add --parallel  back in
#${docker_compose_exe} pull &&
#  ${docker_compose_exe} build cray-smd loader x0c0s1b0 x0c0s2b0 &&
#  ${docker_compose_exe} up -d cray-smd loader x0c0s1b0 x0c0s2b0
network_name=${RANDY}_pcs #check the network name!
# TODO make this dynamic, and not just sleep!
sleep 10 #Hey its a hack, but lets just sleep for a few seconds and make sure everything is up.
#the RIGHT way to do it would be check cray-smd for 'discoveredOK' on the expected endpoints, but a sleep is more convenient, if not lazy.
export DOCKER_BUILDKIT=0
docker build --rm --no-cache --network ${network_name} -f Dockerfile.unittesting.Dockerfile .
test_result=$?

# Clean up
echo "Cleaning up containers..."
if [[ $test_result -ne 0 ]]; then
  echo "Unit tests FAILED!"
  cleanup 1
fi

echo "Unit tests PASSED!"
cleanup 0
