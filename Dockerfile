FROM golang:1.22-bookworm as build

WORKDIR /build

COPY go.mod /build
COPY go.sum /build
COPY ./docker/docker-entrypoint.sh /build/docker-entrypoint.sh
RUN go mod download \
    && apt-get update  \
    && apt-get install -y dumb-init  \
    && chmod +x /build/docker-entrypoint.sh

COPY . .

RUN go build -o openshield .

FROM gcr.io/distroless/base-debian12:nonroot

COPY --from=build /usr/bin/dumb-init /usr/bin/dumb-init
COPY --from=build /bin/sh /bin/sh
COPY --from=build /build/openshield /app/openshield
COPY --from=build /build/docker-entrypoint.sh /app/docker-entrypoint.sh

USER nonroot:nonroot
WORKDIR /app
ENTRYPOINT ["./docker-entrypoint.sh"]

CMD ["/app/openshield","start"]