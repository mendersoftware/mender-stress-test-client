FROM golang:1.11-alpine3.9 as builder
RUN apk update && \
    apk add gcc musl-dev && \
    mkdir -p /go/src/github.com/mendersoftware/mender-stress-test-client
WORKDIR /go/src/github.com/mendersoftware/mender-stress-test-client
ADD ./ .
RUN go build

FROM alpine:3.9
COPY --from=builder /go/src/github.com/mendersoftware/mender-stress-test-client/mender-stress-test-client /
COPY ./entrypoint.sh /
ENTRYPOINT ["/entrypoint.sh"]

