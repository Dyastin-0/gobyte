FROM golang:latest

WORKDIR /app

COPY . .

RUN go build -o build/gobyte ./cli/main.go

EXPOSE 8888 8889

ENTRYPOINT ["./build/gobyte"]
