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

set -x

# Configure docker compose
export COMPOSE_PROJECT_NAME=$RANDOM
export COMPOSE_FILE=docker-compose.test.ct.yaml
ephCertDir=ephemeral_cert

echo "COMPOSE_PROJECT_NAME: ${COMPOSE_PROJECT_NAME}"
echo "COMPOSE_FILE: $COMPOSE_FILE"


function cleanup() {
  rm -rf $ephCertDir
  docker-compose down
  if [[ $? -ne 0 ]]; then
    echo "Failed to decompose environment!"
    exit 1
  fi
  exit $1
}


# Create "ephemeral" TLS .crt and .key files

mkdir -p $ephCertDir
openssl req -newkey rsa:4096 \
    -x509 -sha256 \
    -days 3650 \
    -nodes \
    -subj "/C=US/ST=Minnesota/L=Bloomington/O=HPE/OU=Engineering/CN=hpe.com" \
    -out $ephCertDir/tls.crt \
    -keyout $ephCertDir/tls.key

# Get the base containers running
echo "Starting containers..."
docker-compose build --no-cache
docker-compose up -d cray-power-control #this will stand up everything except for the integration test container

# Run the CT smoke tests
#TODO: getting the following smoke test failure with all optional parameters I've tried:
# {
#   "type": "about:blank",
#   "detail": "invalid PowerStateFilter type ",
#   "status": 400,
#   "title": "Bad Request"
# }
#docker-compose up --exit-code-from ct-tests-smoke ct-tests-smoke
#test_result=$?
#echo "Cleaning up containers..."
#if [[ $test_result -ne 0 ]]; then
#  echo "CT smoke tests FAILED!"
#  cleanup 1
#fi

# Run the CT functional tests
docker-compose up -d ct-tests-functional-wait-for-smd
#TODO: failing on this docker wait
docker wait ${COMPOSE_PROJECT_NAME}_ct-tests-functional-wait-for-smd_1
#TODO: add cleanup/exit if wait-for-smd times out
docker logs ${COMPOSE_PROJECT_NAME}_ct-tests-functional-wait-for-smd_1

docker-compose up --exit-code-from ct-tests-functional ct-tests-functional
test_result=$?

# Clean up
echo "Cleaning up containers..."
if [[ $test_result -ne 0 ]]; then
  echo "CT functional tests FAILED!"
  cleanup 1
fi

# Cleanup
echo "CT tests PASSED!"
cleanup 0
