.PHONY: build test docker-build docker-run deploy lint

# Переменные
BINARY_NAME=devops-manager
DOCKER_IMAGE=devops-manager:latest

# Цель для сборки приложения
build:
	go build -o $(BINARY_NAME) ./cmd/main.go

# Цель для запуска тестов
test:
	go test -v ./...

# Цель для запуска линтера
lint:
	golangci-lint run

# Цель для сборки Docker-образа
docker-build:
	docker build -t $(DOCKER_IMAGE) .

# Цель для запуска Docker-контейнера
docker-run:
	docker run -p 8080:8080 $(DOCKER_IMAGE)

# Цель для деплоя
deploy:
	kubectl apply -f deploy/ 