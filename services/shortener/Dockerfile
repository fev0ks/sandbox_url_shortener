FROM golang:1.25-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /bin/shortener ./services/shortener/cmd/shortener

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /bin/shortener /usr/local/bin/shortener
EXPOSE 8080
ENTRYPOINT ["shortener"]
