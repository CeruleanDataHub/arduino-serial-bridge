#
# BUILD IMAGE
#
FROM --platform=$TARGETPLATFORM golang:1.14.4-alpine AS builder

ARG TARGETPLATFORM
ARG BUILDPLATFORM

RUN apk add --update --no-cache git build-base linux-headers

WORKDIR /build

COPY . .

ENV GO111MODULE=on
ENV TARGET=$TARGETPLATFORM

RUN export PLATFORM=$(echo $TARGET | sed "s/linux\///"); \
    CGO_ENABLED=1 GOOS=linux GOARCH=$PLATFORM \
    go build -a -installsuffix cgo -o arduino-serial-bridge

#
# RELEASE IMAGE
#
FROM --platform=$TARGETPLATFORM alpine:3.12

WORKDIR /root/
COPY --from=builder /build/arduino-serial-bridge .

CMD ["./arduino-serial-bridge"]
