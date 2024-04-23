FROM golang:1.22-alpine

WORKDIR /app

COPY go.mod go.sum ./

COPY . ./
VOLUME [ "/data" ]
EXPOSE 8080

RUN go build ./main.go

CMD ["/app/main"]