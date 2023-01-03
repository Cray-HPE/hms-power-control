#!/usr/bin/env bash

#
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
#
set -x


# Configure docker compose
export COMPOSE_PROJECT_NAME=$RANDOM
export COMPOSE_FILE=docker-compose.test.ct.yaml

echo "COMPOSE_PROJECT_NAME: ${COMPOSE_PROJECT_NAME}"
echo "COMPOSE_FILE: $COMPOSE_FILE"

function cleanup() {
  echo "Cleaning up containers..."
  if ! docker compose down --remove-orphans; then
    echo "Failed to decompose environment!"
    exit 1
  fi
  exit $1
}


# Get the base containers running
echo "Starting containers..."
docker compose build --no-cache
docker compose up -d cray-power-control

#sleep 15

docker compose logs cray-power-control
if ! docker compose up --no-recreate --exit-code-from smoke smoke; then
  echo "CT smoke tests FAILED!"
  cleanup 1
fi

#TODO
## wait for containers to stabilize and simulated HSM hardware discoveries to complete
#if ! docker compose up --exit-code-from wait-for-smd wait-for-smd; then
#  echo "Timed out on HSM waiting for discoveries to complete."
#  cleanup 1
#fi

#TODO
#sleep 60
#sleep 30

# execute the CT functional tests
if ! docker compose up --exit-code-from tavern tavern; then
  echo "CT tavern tests FAILED!"
  cleanup 1
fi

# Cleanup
echo "CT tests PASSED!"
cleanup 0
