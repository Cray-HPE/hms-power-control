# MIT License
#
# (C) Copyright [2022-2023] Hewlett Packard Enterprise Development LP
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

# Dockerfile for building hms-power-control.

# Build base just has the packages installed we need.
FROM artifactory.algol60.net/docker.io/library/golang:1.23-alpine AS build-base

RUN set -ex \
    && apk -U upgrade \
    && apk add build-base

# Base copies in the files we need to test/build.
FROM build-base AS base

RUN go env -w GO111MODULE=auto

# Copy all the necessary files to the image.
COPY cmd $GOPATH/src/github.com/Cray-HPE/hms-power-control/cmd
COPY vendor $GOPATH/src/github.com/Cray-HPE/hms-power-control/vendor
COPY internal $GOPATH/src/github.com/Cray-HPE/hms-power-control/internal
COPY .version $GOPATH/src/github.com/Cray-HPE/hms-power-control/.version

### Build Stage ###
FROM base AS builder

RUN set -ex && go build -v -tags musl -o /usr/local/bin/hms-power-control github.com/Cray-HPE/hms-power-control/cmd/hms-power-control

### Final Stage ###

FROM artifactory.algol60.net/docker.io/alpine:3
LABEL maintainer="Hewlett Packard Enterprise"
EXPOSE 28007
STOPSIGNAL SIGTERM

RUN set -ex \
    && apk -U upgrade \
    && apk add curl jq

# Get the hms-power-control from the builder stage.
COPY --from=builder /usr/local/bin/hms-power-control /usr/local/bin/.
COPY configs configs
COPY .version /


# Setup environment variables.
ENV SMS_SERVER "http://cray-smd:27779"
ENV LOG_LEVEL "INFO"
ENV SERVICE_RESERVATION_VERBOSITY "INFO"
ENV TRS_IMPLEMENTATION "LOCAL"
ENV STORAGE "ETCD"
ENV ETCD_HOST "etcd"
ENV ETCD_PORT "2379"
ENV HSMLOCK_ENABELD "true"
ENV VAULT_ENABLED "true"
ENV VAULT_ADDR="http://vault:8200"
ENV VAULT_KEYPATH="secret/hms-creds"

#DONT USES IN PRODUCTION; MOST WILL BREAK PROD!!!
ENV VAULT_SKIP_VERIFY="true"
ENV VAULT_TOKEN "hms"
ENV CRAY_VAULT_AUTH_PATH "auth/token/create"
ENV CRAY_VAULT_ROLE_FILE "/go/configs/namespace"
ENV CRAY_VAULT_JWT_FILE "/go/configs/token"

#nobody 65534:65534
USER 65534:65534

CMD ["sh", "-c", "hms-power-control"]
