FROM ubuntu:20.04

RUN apt update && \
    apt upgrade -y && \
    apt install -y wget

RUN wget "https://go.dev/dl/go1.20.4.linux-$(dpkg --print-architecture).tar.gz" && \
    rm -rf /usr/local/go && \
    tar -C /usr/local -xzf go*.tar.gz && \
    rm go*.tar.gz
ENV PATH=$PATH:/usr/local/go/bin:/root/go/bin

RUN go install github.com/playwright-community/playwright-go/cmd/playwright@latest && \
    playwright install --with-deps

RUN mkdir /src
WORKDIR /src
COPY . .
RUN go install -ldflags '-w -s' . && \
    roll20mapbot --help
WORKDIR /

ENTRYPOINT ["roll20mapbot"]
