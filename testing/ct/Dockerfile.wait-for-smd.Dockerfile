FROM artifactory.algol60.net/docker.io/library/alpine:3.15

RUN set -x \
    && apk -U upgrade \
    && apk add --no-cache \
        bash \
        curl \
        jq

COPY wait-for.sh /src/app/wait-for.sh

WORKDIR /src/app
# Run as nobody
RUN chown -R 65534:65534 /src
USER 65534:65534

# this is inherited from the hms-test container
CMD [ "/src/app/wait-for.sh" ]