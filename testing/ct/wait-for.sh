#!/bin/bash

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

# wait-for.sh; used by runCT.sh to make sure HSM has been populated with data before running.
echo "Waiting for data in HSM /State/Components..."
URL="http://cray-smd:27779/hsm/v2/State/Components"
sentry=1
limit=200
while :; do
  length=$(curl --silent ${URL} | jq '.Components | length')

  if [ -n "$length" ] && [ "$length" -gt "0" ]; then
    echo "$URL is available"
    break
  fi

  if [ "$sentry" -gt "$limit" ]; then
    echo "Failed to connect for $limit, exiting"
    exit 1
  fi

  ((sentry++))

  echo "$URL is unavailable - sleeping"
  sleep 1

done

# additional wait time for ETCD to actually be ready to serve data,
# ETCD's /health endpoint is not an indicator of this ability
sleep 20
