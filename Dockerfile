FROM golang:1.25-alpine AS builder

ARG SERVICE_PATH
ARG BINARY_NAME

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o /out/${BINARY_NAME} ./${SERVICE_PATH}

FROM alpine:3.21

ARG BINARY_NAME

WORKDIR /app

RUN adduser -D -u 10001 appuser \
    && mkdir -p /app \
    && mkdir -p /data \
    && chown -R appuser:appuser /app /data

COPY --from=builder /out/${BINARY_NAME} /app/service

USER appuser

ENTRYPOINT ["/app/service"]
