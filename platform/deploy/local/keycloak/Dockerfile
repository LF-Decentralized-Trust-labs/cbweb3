FROM quay.io/keycloak/keycloak:26.2.5

USER root

ADD https://github.com/stedolan/jq/releases/download/jq-1.6/jq-linux64 /usr/local/bin/jq

RUN chmod +x /usr/local/bin/jq

ADD https://github.com/moparisthebest/static-curl/releases/download/v8.4.0/curl-amd64 /usr/local/bin/curl

RUN chmod +x /usr/local/bin/curl

USER keycloak
