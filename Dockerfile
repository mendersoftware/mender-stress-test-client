FROM golang:1.16.3 as builder
RUN apt-get update -qqy && apt-get install -qqy gcc libssl-dev
RUN mkdir -p /go/src/github.com/mendersoftware/mender-stress-test-client
WORKDIR /go/src/github.com/mendersoftware/mender-stress-test-client
ADD ./ .
RUN go build

FROM alpine:3.13.4
COPY --from=builder /go/src/github.com/mendersoftware/mender-stress-test-client/mender-stress-test-client /
RUN apk add --no-cache ca-certificates libc6-compat && update-ca-certificates
ENTRYPOINT ["/mender-stress-test-client"]
