# --- Stage 1: Build ---
    FROM golang:1.23 AS builder

    WORKDIR /app
    
    # скачиваем зависимости
    COPY go.mod go.sum ./
    RUN go mod download
    
    # копируем код
    COPY . .
    
    # собираем бинарник
    RUN go build -o app main.go
    
    # --- Stage 2: Minimal runtime ---
    FROM alpine:latest
    
    WORKDIR /app
    
    # копируем бинарник и необходимые файлы
    COPY --from=builder /app/app .
    COPY --from=builder /app/.env .
    COPY --from=builder /app/docs ./docs
    
    # убедимся, что бинарник исполняемый
    RUN chmod +x ./app
    
    # иногда нужно для Go бинарников с CGO
    RUN apk add --no-cache libc6-compat
    
    EXPOSE 8080
    
    CMD ["./app"]
    