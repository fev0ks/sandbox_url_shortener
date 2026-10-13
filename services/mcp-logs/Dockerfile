FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/mcp-logs ./services/mcp-logs/cmd/mcp-logs

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 mcp
COPY --from=builder /bin/mcp-logs /usr/local/bin/mcp-logs
USER mcp
EXPOSE 8090
ENTRYPOINT ["mcp-logs"]
