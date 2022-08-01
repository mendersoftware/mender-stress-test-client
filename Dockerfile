FROM golang:1.18.2 as builder
RUN mkdir -p /go/src/github.com/mendersoftware/mender-stress-test-client
WORKDIR /go/src/github.com/mendersoftware/mender-stress-test-client
ADD ./ .
RUN go build

FROM alpine:3.16.1
COPY --from=builder /go/src/github.com/mendersoftware/mender-stress-test-client/mender-stress-test-client /
RUN apk add --no-cache ca-certificates libc6-compat && update-ca-certificates
ENTRYPOINT ["/mender-stress-test-client"]
