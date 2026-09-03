FROM alpine:3.23

RUN apk add --no-cache \
  curl \
  docker-cli \
  bash \
  git \
  make

RUN curl -s https://raw.githubusercontent.com/nektos/act/master/install.sh | sh

RUN mv bin/act /usr/local/bin/act

WORKDIR /src

ENTRYPOINT ["act"]
