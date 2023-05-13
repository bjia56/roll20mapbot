FROM ubuntu:20.04

RUN apt update && \
    apt upgrade -y && \
    apt install -y wget

RUN wget https://go.dev/dl/go1.20.4.linux-amd64.tar.gz && \
    rm -rf /usr/local/go && \
    tar -C /usr/local -xzf go1.20.4.linux-amd64.tar.gz && \
    rm go*.tar.gz
ENV PATH=$PATH:/usr/local/go/bin:/root/go/bin

RUN go install github.com/playwright-community/playwright-go/cmd/playwright@latest && \
    playwright install --with-deps

COPY . .
RUN go install .

ENTRYPOINT roll20mapbot