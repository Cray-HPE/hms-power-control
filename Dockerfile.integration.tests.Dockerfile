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

# This file only exists as a means to run tests in an automated fashion.

FROM artifactory.algol60.net/docker.io/library/alpine:3.13 AS build-base

ENV LOG_LEVEL TRACE
ENV API_URL "http://power-control"
ENV API_SERVER_PORT ":28007"
ENV API_BASE_PATH ""
ENV VERIFY_SSL False


COPY test/integration/py/Pipfile* /
COPY test/integration/py/src src
COPY test/integration/py/requirements.txt .

RUN set -x \
    && apk -U upgrade \
    && apk add --no-cache \
        bash \
        curl \
        python3 \
        py3-pip \
    && pip3 install --upgrade pip \
    && pip3 install \
        requests \
        pytest

WORKDIR src

# PROTIP: python -m pytest test/ is different than pytest test/
# the first one appends some path stuff and python paths are a PITA; so DONT change this!
RUN set -ex \
    && pwd \
    && python3 -m pytest test/

#in case you want to sleep instead of RUN
#CMD ["sh", "-c", "sleep 1000" ]

#build and run
#docker build --rm --no-cache --network hms-power-control_rts -f test.Dockerfile .

#build then run-
#docker build -t fas_test -f test.Dockerfile .
#docker run -d --name fas_test --network hms-power-control_rts fas_test