package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/localops/devops-manager/internal/adapters/cicd"
	"github.com/localops/devops-manager/internal/adapters/docker"
	"github.com/localops/devops-manager/internal/adapters/kubernetes"
	"github.com/localops/devops-manager/internal/adapters/monitoring"
	"github.com/localops/devops-manager/pkg/api"
)

func main() {
	// Инициализация Docker Registry конфигурации
	registryConfig := &docker.RegistryConfig{
		URL:      os.Getenv("DOCKER_REGISTRY_URL"),
		Username: os.Getenv("DOCKER_REGISTRY_USERNAME"),
		Password: os.Getenv("DOCKER_REGISTRY_PASSWORD"),
		Insecure: os.Getenv("DOCKER_REGISTRY_INSECURE") == "true",
	}

	// Инициализация адаптеров
	dockerAdapter, err := docker.NewDockerAdapter(registryConfig)
	if err != nil {
		log.Fatalf("Failed to initialize Docker adapter: %v", err)
	}

	// Получаем путь к конфигурации Kubernetes
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Failed to get user home directory: %v", err)
	}
	kubeconfigPath := filepath.Join(homeDir, ".kube", "config")

	// Проверяем наличие конфигурационного файла
	if _, err := os.Stat(kubeconfigPath); os.IsNotExist(err) {
		log.Printf("Warning: Kubernetes config file not found at %s", kubeconfigPath)
		log.Printf("Please ensure you have Kubernetes configured properly")
		log.Printf("You can set KUBERNETES_MASTER environment variable or create config file")
	}

	// Устанавливаем переменную окружения KUBERNETES_MASTER, если она не установлена
	if os.Getenv("KUBERNETES_MASTER") == "" {
		os.Setenv("KUBERNETES_MASTER", "http://localhost:8080")
	}

	k8sAdapter, err := kubernetes.NewK8sAdapter(kubeconfigPath)
	if err != nil {
		log.Fatalf("Failed to initialize Kubernetes adapter: %v", err)
	}

	ciAdapter := cicd.NewCICDAdapter(cicd.Config{
		BaseURL: "https://gitlab.example.com/api/v4",
		Token:   "your-token",
	})

	monitoringAdapter := monitoring.NewMonitoringAdapter(monitoring.Config{
		Namespace: "devops",
		Subsystem: "manager",
	})

	// Инициализация API
	handler := api.NewAPI(dockerAdapter, k8sAdapter, ciAdapter, monitoringAdapter)

	// Настройка HTTP сервера
	srv := &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}

	// Запуск сервера в горутине
	go func() {
		log.Printf("Starting server on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Ожидание сигнала для graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// Graceful shutdown
	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}
