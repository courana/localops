# Этап сборки
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Копируем go.mod и go.sum
COPY go.mod go.sum ./

# Загружаем зависимости
RUN go mod download

# Копируем исходный код
COPY . .

# Собираем бинарный файл
RUN CGO_ENABLED=0 GOOS=linux go build -o devops-manager ./cmd/main.go

# Этап финального образа
FROM scratch

WORKDIR /app

# Копируем бинарный файл из этапа сборки
COPY --from=builder /app/devops-manager .

# Запускаем приложение
CMD ["./devops-manager"] 