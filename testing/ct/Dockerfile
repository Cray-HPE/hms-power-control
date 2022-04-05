FROM artifactory.algol60.net/csm-docker/stable/hms-test:3.0.0

COPY smoke/ /src/app
COPY functional/ /src/app
COPY tavern_global_config_ct_test.yaml /src/app/tavern_global_config_ct_test.yaml

ENV PATH="/src/libs:${PATH}"
ENV PATH="/src/app:${PATH}"

USER root
RUN chown -R 65534:65534 /src
USER 65534:65534

# this is inherited from the hms-test container
ENTRYPOINT [ "entrypoint.sh" ]