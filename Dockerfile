FROM golang:1.25.1-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY frontend ./frontend

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/rdc ./cmd/api

FROM alpine:3.22

RUN addgroup -S app && adduser -S app -G app && apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /out/rdc /app/rdc
COPY --from=builder /src/frontend /app/frontend

EXPOSE 8080

USER app

ENTRYPOINT ["/app/rdc"]
