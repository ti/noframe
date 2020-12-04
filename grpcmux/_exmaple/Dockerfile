FROM golang:1.15-alpine as builder
ADD . $GOPATH/src/github.com/ti/noframe/grpcmux/_exmaple
WORKDIR $GOPATH/src/github.com/ti/noframe/grpcmux/_exmaple
RUN CGO_ENABLED=0 go build -ldflags '-s -w' -o /app/main

FROM scratch
COPY --from=builder /app /app/
WORKDIR /app
CMD ["/app/main"]
EXPOSE 8080 8081
