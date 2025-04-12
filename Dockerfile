FROM golang:latest

WORKDIR /app

COPY . .

RUN go build -o build/gobyte gobyte/main.go

EXPOSE 8888 8889

ENTRYPOINT ["./build/gobyte"]
