#!/usr/bin/env bash

# MIT License
#
# (C) Copyright [2022] Hewlett Packard Enterprise Development LP
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

DASHN=false
# parse command-line options
while getopts "hn" opt; do
    case ${opt} in
        h) echo "Usage: runCT.sh [-h] [-n]"
           echo
           echo "Arguments:"
           echo "    -h        display this help message"
           echo "    -n        no cleanup; leaves docker-compose environment running"
           echo "              for manual debugging, requires manual cleanup"
           exit 0
           ;;
        n) DASHN=true
           ;;
    esac
done

set -x

# Configure docker compose
export COMPOSE_PROJECT_NAME=$RANDOM
export COMPOSE_FILE=docker-compose.test.ct.yaml
ephCertDir=ephemeral_cert

echo "COMPOSE_PROJECT_NAME: ${COMPOSE_PROJECT_NAME}"
echo "COMPOSE_FILE: $COMPOSE_FILE"


function cleanup() {
  if ! ${DASHN}; then
    rm -rf $ephCertDir
    docker-compose down
    if [[ $? -ne 0 ]]; then
      echo "Failed to decompose environment!"
      exit 1
    fi
  else
    echo "Exiting without cleanup"
  fi
  exit $1
}


# Create "ephemeral" TLS .crt and .key files

mkdir -p $ephCertDir
openssl req -newkey rsa:4096 \
    -x509 -sha256 \
    -days 1 \
    -nodes \
    -subj "/C=US/ST=Minnesota/L=Bloomington/O=HPE/OU=Engineering/CN=hpe.com" \
    -out $ephCertDir/rts.crt \
    -keyout $ephCertDir/rts.key

# When running in github actions the rts.key gets created with u+rw permissions.
# This prevents RTS which runs as nobody to read the key as it doesn't have permission.
chmod o+r $ephCertDir/rts.crt $ephCertDir/rts.key

# Get the base containers running
echo "Starting containers..."
docker-compose build --no-cache
docker-compose up -d cray-power-control #this will stand up everything except for the integration test container

# Give PCS, HSM, and ETCD time to be fully initialized before running CT tests
docker-compose up -d ct-tests-functional-wait-for-smd
docker wait ${COMPOSE_PROJECT_NAME}_ct-tests-functional-wait-for-smd_1
DOCKER_WAIT_FOR_SMD_LOGS=$(docker logs ${COMPOSE_PROJECT_NAME}_ct-tests-functional-wait-for-smd_1)
DOCKER_WAIT_CHECK=$(echo "${DOCKER_WAIT_FOR_SMD_LOGS}" | grep -E "Failed to connect for [0-9]+, exiting")
if [[ -n "${DOCKER_WAIT_CHECK}" ]]; then
    echo "Timed out waiting for HSM to be populated, exiting..."
    cleanup 1
fi

# Run the CT smoke tests
docker-compose up --exit-code-from ct-tests-smoke ct-tests-smoke
test_result=$?
echo "Cleaning up containers..."
if [[ $test_result -ne 0 ]]; then
  echo "CT smoke tests FAILED!"
  cleanup 1
fi

# Run the CT functional tests
docker-compose up --exit-code-from ct-tests-functional ct-tests-functional
test_result=$?

# Cleanup
echo "Cleaning up containers..."
if [[ $test_result -ne 0 ]]; then
  echo "CT functional tests FAILED!"
  cleanup 1
fi

# Cleanup
echo "CT tests PASSED!"
cleanup 0
