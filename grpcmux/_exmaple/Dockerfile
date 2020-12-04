FROM golang:1.15-alpine
ADD . $GOPATH/src/github.com/ti/noframe/grpcmux/_exmaple
WORKDIR $GOPATH/src/github.com/ti/noframe/grpcmux/_exmaple
RUN go build -ldflags '-s -w' -o /app/main

FROM scratch
WORKDIR /app
COPY --from=0 /app/main .
CMD ["/app/main"]
EXPOSE 8080 8081 
