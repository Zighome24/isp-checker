FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o isp-checker .

FROM alpine:latest

RUN apk --no-cache add ca-certificates
WORKDIR /app

COPY --from=builder /app/isp-checker .
COPY config.yaml .

EXPOSE 9090

CMD ["./isp-checker", "config.yaml"]