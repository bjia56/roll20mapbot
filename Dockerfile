FROM ubuntu:20.04

ENV DEBIAN_FRONTEND=noninteractive
ENV TZ=Etc/UTC
RUN apt update && \
    apt upgrade -y && \
    apt install -y wget xvfb cups-pdf

RUN wget -q "https://go.dev/dl/go1.20.4.linux-$(dpkg --print-architecture).tar.gz" && \
    rm -rf /usr/local/go && \
    tar -C /usr/local -xzf go*.tar.gz && \
    rm go*.tar.gz
ENV PATH=$PATH:/usr/local/go/bin:/root/go/bin

RUN go install github.com/playwright-community/playwright-go/cmd/playwright@v0.2000.1 && \
    playwright install --with-deps

RUN mkdir /src
RUN mkdir /root/Downloads
WORKDIR /src
COPY . .
RUN go install -ldflags '-w -s' . && \
    roll20mapbot --help
WORKDIR /
COPY run.sh /usr/local/bin/run.sh

ENTRYPOINT ["run.sh"]
