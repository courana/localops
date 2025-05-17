# DevOps Manager

## Описание проекта
DevOps Manager — это микросервис, разработанный на Go, который предоставляет API для управления Docker-контейнерами, Kubernetes-ресурсами и CI/CD процессами. Он позволяет автоматизировать и упростить управление инфраструктурой и процессами разработки.

## Инструкция по сборке и запуску

### Сборка
1. Убедитесь, что у вас установлен Go (версия 1.21 или выше).
2. Клонируйте репозиторий:
   ```bash
   git clone https://github.com/localops/devops-manager.git
   cd devops-manager
   ```
3. Соберите проект:
   ```bash
   go build -o devops-manager ./cmd/main.go
   ```

### Запуск
1. Запустите приложение:
   ```bash
   ./devops-manager
   ```

### Примеры curl
- **Проверка доступности Docker API**:
  ```bash
  curl -X GET http://localhost:8080/api/docker/ping
  ```
- **Проверка доступности Kubernetes API**:
  ```bash
  curl -X GET http://localhost:8080/api/k8s/ping
  ```
- **Проверка доступности CI/CD API**:
  ```bash
  curl -X GET http://localhost:8080/api/ci/ping
  ```

## Описание config.yaml
Файл `config.yaml` содержит конфигурацию для DevOps Manager. Пример структуры:

```yaml
docker:
  host: "unix:///var/run/docker.sock"
  version: "v1.41"

kubernetes:
  config: "/path/to/kubeconfig"
  namespace: "default"

cicd:
  base_url: "https://gitlab.example.com/api/v4"
  token: "your-token"
```

### Параметры
- **docker**: настройки для Docker API.
  - `host`: адрес Docker API.
  - `version`: версия Docker API.
- **kubernetes**: настройки для Kubernetes API.
  - `config`: путь к файлу конфигурации Kubernetes.
  - `namespace`: пространство имен для работы.
- **cicd**: настройки для CI/CD API.
  - `base_url`: базовый URL для API CI/CD.
  - `token`: токен для аутентификации.

Если нужно добавить дополнительные инструкции или примеры, дайте знать! 