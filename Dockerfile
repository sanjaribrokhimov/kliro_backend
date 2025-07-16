# --- Stage 1: Build ---
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Копируем go.mod и go.sum и устанавливаем зависимости
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходники
COPY . .

# Собираем бинарник
RUN go build -o app main.go

# --- Stage 2: Minimal runtime ---
FROM alpine:latest
WORKDIR /app

# Копируем бинарник и .env
COPY --from=builder /app/app .
COPY --from=builder /app/.env .
COPY --from=builder /app/docs ./docs

EXPOSE 8080

CMD ["./app"] 