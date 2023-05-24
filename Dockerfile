FROM golang:1.20.4-bullseye


RUN go install github.com/playwright-community/playwright-go/cmd/playwright@latest && \
    playwright install --with-deps

RUN mkdir /src
WORKDIR /src
COPY . .
RUN go install .
WORKDIR /

ENTRYPOINT ["roll20mapbot"]
