FROM golang:1.23 AS builder

WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/robot-d ./cmd/robot-d

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=builder /out/robot-d /app/robot-d

EXPOSE 7004

ENTRYPOINT ["/app/robot-d"]
