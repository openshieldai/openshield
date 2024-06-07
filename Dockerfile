FROM golang:1.22-bookworm as build

WORKDIR /build

COPY go.mod /build
COPY go.sum /build
RUN go mod download \
    && apt-get update  \
    && apt-get install -y dumb-init

COPY . .

RUN go build -o openshield .

FROM gcr.io/distroless/base-debian12:nonroot

COPY --from=build /usr/bin/dumb-init /usr/bin/dumb-init
COPY --from=build /bin/sh /bin/sh
COPY --from=build /build/openshield /app/openshield
USER nonroot:nonroot
# https://github.com/gofiber/fiber/issues/1021
# https://github.com/gofiber/fiber/issues/1036
ENTRYPOINT ["/usr/bin/dumb-init", "--"]
WORKDIR /app

CMD ./openshield