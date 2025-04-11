FROM golang:latest

WORKDIR /app

COPY . .

RUN go build -o gobyte cmd/main.go

EXPOSE 8888 8889

ENTRYPOINT ["./gobyte"]
