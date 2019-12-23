FROM golang:1.13 as builder

# local vendor mod
ENV GO111MODULE=off

WORKDIR /go/src/github.com/chenjiandongx/kslb
# build
ADD . /go/src/github.com/chenjiandongx/kslb
RUN go build ./cmd/main.go


FROM nginx

COPY --from=builder /go/src/github.com/chenjiandongx/kslb/main /main
RUN chmod +x /main

WORKDIR /
ENTRYPOINT ["/main"]
