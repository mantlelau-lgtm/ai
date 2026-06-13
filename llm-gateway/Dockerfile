FROM golang:1.23 AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/llm-gateway ./cmd/llm-gateway

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=builder /out/llm-gateway /app/llm-gateway
EXPOSE 50080
ENTRYPOINT ["/app/llm-gateway"]
