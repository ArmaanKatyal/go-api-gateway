FROM golang:1.22 as builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o ./bin/server ./server

EXPOSE 8080

CMD ["/app/bin/server"]
