FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o /out/identity ./services/identity/cmd/identity

FROM alpine:3.21

RUN adduser -D -H -u 10001 appuser
USER appuser

COPY --from=build /out/identity /usr/local/bin/identity

EXPOSE 8081

ENTRYPOINT ["/usr/local/bin/identity"]
