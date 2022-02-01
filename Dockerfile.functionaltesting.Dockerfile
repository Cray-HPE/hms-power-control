# MIT License
#
# (C) Copyright [2020-2021] Hewlett Packard Enterprise Development LP
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

## This file only exists as a means to run tests in an automated fashion.

### Build python base ###

FROM artifactory.algol60.net/docker.io/library/alpine:3.13 AS build-base

RUN set -ex \
    && apk -U upgrade \
    && apk add --no-cache \
        bash \ 
        curl \ 
        python3 \
        py3-pip \
    && pip3 install --upgrade pip \
    && pip3 install \
        pipenv \
        requests \
        pytest

ENV SMS_SERVER "http://cray-smd:27779"
ENV LOG_LEVEL "INFO"
ENV TRS_IMPLEMENTATION "LOCAL"
ENV STORAGE "BOTH"
ENV ETCD_HOST "etcd"
ENV ETCD_PORT "2379"
ENV HSMLOCK_ENABELD "true"
ENV SERVICE_RESERVATION_VERBOSITY "ERROR"


ENV VAULT_TOKEN "hms"
ENV VAULT_ENABLED "true"
ENV VAULT_ADDR="http://vault:8200"
ENV VAULT_SKIP_VERIFY="true"
ENV VAULT_KEYPATH="secret/hms-creds"
ENV CRAY_VAULT_AUTH_PATH "auth/token/create"
ENV CRAY_VAULT_ROLE_FILE "/go/configs/namespace"
ENV CRAY_VAULT_JWT_FILE "/go/configs/token"

ENV API_URL "http://power-control"
ENV API_SERVER_PORT ":28007"

COPY cmd $GOPATH/src/github.com/Cray-HPE/hms-power-control/cmd
COPY configs configs
COPY vendor $GOPATH/src/github.com/Cray-HPE/hms-power-control/vendor
COPY internal $GOPATH/src/github.com/Cray-HPE/hms-power-control/internal
COPY .version $GOPATH/src/github.com/Cray-HPE/hms-power-control/.version
COPY test/functional /test/functional
COPY test/integration /test/integration

# if you use CMD, then it will run like a service; we want this to execute the tests and quit
#RUN go test -v ./...
RUN set -ex \
  && pwd \
  && cd test/functional \
  && python3 -m pytest . 
