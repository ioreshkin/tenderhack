FROM golang:1.25-bookworm AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/tenderhack ./cmd/tenderhack

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    chromium \
    fonts-dejavu-core \
    postgresql-client \
    tzdata \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=build /out/tenderhack /app/tenderhack
COPY TenderHack_*.xlsx /app/
COPY docker/entrypoint.sh /entrypoint.sh

RUN chmod +x /entrypoint.sh && mkdir -p /app/generated

ENV DATABASE_URL=postgres://postgres:postgres@postgres:5432/tenderhack?sslmode=disable
ENV HTTP_ADDR=:8080
ENV DOCS_DIR=/app/generated
ENV PDF_BROWSER=/usr/bin/chromium
ENV AUTO_IMPORT=1

EXPOSE 8080

ENTRYPOINT ["/entrypoint.sh"]
CMD ["serve"]
