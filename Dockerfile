FROM golang:1.23.4 AS build

WORKDIR /go/src/app

COPY . .

RUN env GOOS=linux GARCH=amd64 CGO_ENABLED=0 go build -o app

FROM alpine:3.7

COPY --from=build /go/src/app/app /usr/local/bin/s3-web

CMD ["s3-web"]