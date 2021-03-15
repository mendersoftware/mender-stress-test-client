FROM golang:1.16.2-alpine3.12 as builder
RUN apk add --no-cache gcc musl-dev openssl-dev
RUN mkdir -p /go/src/github.com/mendersoftware/mender-stress-test-client
WORKDIR /go/src/github.com/mendersoftware/mender-stress-test-client
ADD ./ .
RUN go build

FROM alpine:3.13.2
COPY --from=builder /go/src/github.com/mendersoftware/mender-stress-test-client/mender-stress-test-client /
RUN apk add --no-cache ca-certificates && update-ca-certificates
ENTRYPOINT ["/mender-stress-test-client"]
