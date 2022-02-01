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

# This file only exists as a means to run tests in an automated fashion.

FROM artifactory.algol60.net/docker.io/library/alpine:3.13 AS build-base

ENV LOG_LEVEL TRACE
ENV API_URL "http://cray-power-control"
ENV API_SERVER_PORT ":28007"
ENV API_BASE_PATH ""
ENV VERIFY_SSL False

RUN set -x \
    && apk -U upgrade \
    && apk add --no-cache \
        bash \
        curl

CMD ["sh", "-c", "curl --fail -i ${API_URL}${API_SERVER_PORT}/" ]

    # ^right now we dont have any integration tests, so this is more perfunctary