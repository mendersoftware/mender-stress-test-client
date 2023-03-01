FROM golang:1.19.5 as builder
RUN mkdir -p /go/src/github.com/mendersoftware/mender-stress-test-client
WORKDIR /go/src/github.com/mendersoftware/mender-stress-test-client
ADD ./ .
RUN go build

FROM alpine:3.17.2
COPY --from=builder /go/src/github.com/mendersoftware/mender-stress-test-client/mender-stress-test-client /
RUN apk add --no-cache ca-certificates libc6-compat && update-ca-certificates
ENTRYPOINT ["/mender-stress-test-client"]
