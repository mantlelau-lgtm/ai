FROM golang:1.23 AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/message-gateway ./cmd/message-gateway

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=builder /out/message-gateway /app/message-gateway
EXPOSE 50082
ENTRYPOINT ["/app/message-gateway"]
