FROM golang:1.25-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /bin/analytics ./services/analytics/cmd/analytics

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /bin/analytics /usr/local/bin/analytics
EXPOSE 9090
ENTRYPOINT ["analytics"]
