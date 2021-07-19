FROM --platform=$BUILDPLATFORM golang:alpine as builder

ARG TARGETARCH
ENV GOARCH=$TARGETARCH

RUN apk add --no-cache git
COPY . /go/src/github.com/tsuru/networkapi-ingress-controller
WORKDIR /go/src/github.com/tsuru/networkapi-ingress-controller
ENV GO111MODULE=on
RUN CGO_ENABLED=0 go build -ldflags "-X github.com/tsuru/networkapi-ingress-controller/main.GitHash=`git rev-parse HEAD`" .

FROM alpine

RUN apk add --no-cache ca-certificates
COPY --from=builder /go/src/github.com/tsuru/networkapi-ingress-controller/networkapi-ingress-controller /bin/networkapi-ingress-controller
ENTRYPOINT ["/bin/networkapi-ingress-controller"]
