FROM golang:latest

WORKDIR /app

COPY . .

RUN go build -o build/gobyte ./cli/cli.go

EXPOSE 8080 42069
 
ENTRYPOINT ["./build/gobyte"]
