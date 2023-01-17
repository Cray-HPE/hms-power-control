#!/usr/bin/env bash
# MIT License
#
# (C) Copyright [2020-2023] Hewlett Packard Enterprise Development LP
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

IFS=','

vault_host_port=$(echo "$VAULT_ADDR" | awk -F[/] '{print $3}')
vault_host=$(echo "$vault_host_port" | cut -d ':' -f 1)
vault_port=$(echo "$vault_host_port" | cut -d ':' -f 2)

echo "Waiting for Vault ($vault_host:$vault_port) to become ready..."

./wait-for.sh "$vault_host":"$vault_port" -- echo 'Vault ready.'

set -ex

vault login "$VAULT_TOKEN"

# Rename the v2 kv-store, so we can create a v1 kv-store named secret
vault secrets move secret/ secret-v2/

read -ra STORE <<<"$KV_STORES"
for store in "${STORE[@]}"; do
  echo "Enabling $store..."
  vault secrets enable -path="$store" kv
done

exit 0

