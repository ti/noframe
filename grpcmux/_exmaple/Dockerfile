FROM golang:1.15
ADD . $GOPATH/src/github.com/ti/noframe/grpcmux/_exmaple
WORKDIR $GOPATH/src/github.com/ti/noframe/grpcmux/_exmaple
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/main .

FROM scratch
WORKDIR /app
COPY --from=0 /app/main .
CMD ["/app/main"]
EXPOSE 8080 8081