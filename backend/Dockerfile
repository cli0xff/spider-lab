FROM golang:1.25-alpine AS builder

ARG TARGET=master

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server ./cmd/${TARGET}

FROM python:3.12-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates nodejs npm git curl \
    chromium fonts-freefont-ttf \
    && rm -rf /var/lib/apt/lists/* \
    && pip install --no-cache-dir playwright "playwright-stealth>=2.0.0" httpx parsel tenacity requests \
    && PLAYWRIGHT_BROWSERS_PATH=/ms-playwright python -m playwright install chromium || true

ENV PLAYWRIGHT_BROWSERS_PATH=/ms-playwright

WORKDIR /app

COPY --from=builder /app/server .
COPY scripts/ /app/scripts/

RUN mkdir -p /app/spiders

EXPOSE 8080 9666

CMD ["./server"]
