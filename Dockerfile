FROM golang1.22:alpine
WORKDIR /app
COPY go.* ./
RUN go mod download
COPY . ./
RUN go build main.go
CMD [ "./main" ]
