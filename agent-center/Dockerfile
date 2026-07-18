FROM golang:1.23 AS build

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/agent-center ./cmd/agent-center

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=build /out/agent-center /app/agent-center

EXPOSE 9999

ENTRYPOINT ["/app/agent-center"]
