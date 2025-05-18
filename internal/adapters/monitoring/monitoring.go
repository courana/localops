package monitoring

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Config содержит конфигурацию для Monitoring адаптера
type Config struct {
	Namespace string
	Subsystem string
	Port      int
}

// MetricValue представляет значение метрики
type MetricValue struct {
	Name      string
	Value     float64
	Timestamp time.Time
	Labels    map[string]string
}

// HealthCheck представляет результат проверки здоровья сервиса
type HealthCheck struct {
	Name      string
	Status    string
	Message   string
	Timestamp time.Time
}

// MonitoringAdapter предоставляет методы для работы с системой мониторинга
type MonitoringAdapter struct {
	config Config
	// Реестр метрик
	registry *prometheus.Registry
	// Счетчики
	counters map[string]*prometheus.CounterVec
	// Гистограммы
	histograms map[string]*prometheus.HistogramVec
	// HTTP сервер
	server *http.Server
}

// NewMonitoringAdapter создает новый экземпляр MonitoringAdapter
func NewMonitoringAdapter(config Config) *MonitoringAdapter {
	adapter := &MonitoringAdapter{
		config:     config,
		registry:   prometheus.NewRegistry(),
		counters:   make(map[string]*prometheus.CounterVec),
		histograms: make(map[string]*prometheus.HistogramVec),
	}

	// Регистрируем метрики для Docker операций
	adapter.RegisterCounters(
		[]string{
			"docker_operations_total",
			"docker_image_operations_total",
			"docker_container_operations_total",
			"docker_network_operations_total",
		},
		[]string{"operation", "status"},
	)

	// Регистрируем метрики для Kubernetes операций
	adapter.RegisterCounters(
		[]string{
			"kubernetes_operations_total",
			"kubernetes_resource_operations_total",
		},
		[]string{"operation", "resource_type", "status"},
	)

	// Регистрируем метрики для CI/CD операций
	adapter.RegisterCounters(
		[]string{
			"cicd_operations_total",
			"cicd_pipeline_operations_total",
		},
		[]string{"operation", "status"},
	)

	// Регистрируем гистограммы для длительности операций
	adapter.RegisterHistograms(
		[]string{
			"docker_operation_duration_seconds",
			"kubernetes_operation_duration_seconds",
			"cicd_operation_duration_seconds",
		},
		[]string{"operation"},
		[]float64{0.1, 0.5, 1.0, 2.0, 5.0},
	)

	// Запускаем HTTP сервер для метрик
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(adapter.registry, promhttp.HandlerOpts{}))

	adapter.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", config.Port),
		Handler: mux,
	}

	go func() {
		if err := adapter.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Ошибка запуска HTTP сервера: %v\n", err)
		}
	}()

	return adapter
}

// RegisterCounters регистрирует счетчики с заданными именами и метками
func (a *MonitoringAdapter) RegisterCounters(names []string, labels []string) {
	for _, name := range names {
		a.counters[name] = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: a.config.Namespace,
				Subsystem: a.config.Subsystem,
				Name:      name,
				Help:      "Counter " + name,
			},
			labels,
		)
		a.registry.MustRegister(a.counters[name])
	}
}

// RegisterHistograms регистрирует гистограммы с заданными именами и метками
func (a *MonitoringAdapter) RegisterHistograms(names []string, labels []string, buckets []float64) {
	for _, name := range names {
		a.histograms[name] = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: a.config.Namespace,
				Subsystem: a.config.Subsystem,
				Name:      name,
				Help:      "Histogram " + name,
				Buckets:   buckets,
			},
			labels,
		)
		a.registry.MustRegister(a.histograms[name])
	}
}

// IncCounter увеличивает значение счетчика
func (a *MonitoringAdapter) IncCounter(name string, labels map[string]string) {
	if counter, ok := a.counters[name]; ok {
		counter.With(labels).Inc()
	}
}

// ObserveDuration записывает длительность в гистограмму
func (a *MonitoringAdapter) ObserveDuration(name string, duration time.Duration, labels map[string]string) {
	if histogram, ok := a.histograms[name]; ok {
		histogram.With(labels).Observe(duration.Seconds())
	}
}

// MetricsHandler возвращает HTTP-обработчик для метрик Prometheus
func (a *MonitoringAdapter) MetricsHandler() http.Handler {
	return promhttp.HandlerFor(a.registry, promhttp.HandlerOpts{})
}

// GetRawMetrics возвращает "сырые" метрики
func (m *MonitoringAdapter) GetRawMetrics(ctx context.Context) (string, error) {
	// Делаем HTTP запрос к локальному эндпоинту метрик
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/metrics", m.config.Port))
	if err != nil {
		return "", fmt.Errorf("ошибка при получении метрик: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("неожиданный статус ответа: %d", resp.StatusCode)
	}

	// Читаем тело ответа
	metrics, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ошибка при чтении метрик: %v", err)
	}

	return string(metrics), nil
}

// QueryMetric возвращает значение метрики за указанный период
func (m *MonitoringAdapter) QueryMetric(ctx context.Context, name string, start, end time.Time) ([]MetricValue, error) {
	// Получаем все метрики
	metrics, err := m.GetRawMetrics(ctx)
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении метрик: %v", err)
	}

	// Парсим метрики
	var values []MetricValue
	lines := strings.Split(metrics, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, name) && !strings.HasPrefix(line, "#") {
			parts := strings.Split(line, " ")
			if len(parts) >= 2 {
				value, err := strconv.ParseFloat(parts[1], 64)
				if err != nil {
					continue
				}

				// Извлекаем метки из строки метрики
				labels := make(map[string]string)
				if strings.Contains(line, "{") {
					labelsStr := strings.Split(strings.Split(line, "{")[1], "}")[0]
					labelPairs := strings.Split(labelsStr, ",")
					for _, pair := range labelPairs {
						kv := strings.Split(pair, "=")
						if len(kv) == 2 {
							labels[kv[0]] = strings.Trim(kv[1], "\"")
						}
					}
				}

				values = append(values, MetricValue{
					Name:      name,
					Value:     value,
					Timestamp: time.Now(),
					Labels:    labels,
				})
			}
		}
	}

	if len(values) == 0 {
		return nil, fmt.Errorf("метрика %s не найдена", name)
	}

	return values, nil
}

// ListMetrics возвращает список зарегистрированных метрик
func (m *MonitoringAdapter) ListMetrics(ctx context.Context) ([]string, error) {
	// Здесь должна быть реализация получения списка метрик
	// Для примера возвращаем заглушку
	return []string{
		"http_requests_total",
		"http_request_duration_seconds",
		"cpu_usage_percent",
		"memory_usage_bytes",
		"disk_usage_percent",
	}, nil
}

// GetServiceHealth возвращает результаты проверки здоровья сервиса
func (m *MonitoringAdapter) GetServiceHealth(ctx context.Context) ([]HealthCheck, error) {
	// Здесь должна быть реализация проверки здоровья
	// Для примера возвращаем заглушку
	return []HealthCheck{
		{
			Name:      "API",
			Status:    "healthy",
			Message:   "Service is responding normally",
			Timestamp: time.Now(),
		},
		{
			Name:      "Database",
			Status:    "healthy",
			Message:   "Connection is stable",
			Timestamp: time.Now(),
		},
		{
			Name:      "Cache",
			Status:    "degraded",
			Message:   "High latency detected",
			Timestamp: time.Now(),
		},
	}, nil
}

// RecordDockerOperation записывает метрики для Docker операций
func (a *MonitoringAdapter) RecordDockerOperation(operation string, status string, duration time.Duration) {
	a.IncCounter("docker_operations_total", map[string]string{
		"operation": operation,
		"status":    status,
	})
	a.ObserveDuration("docker_operation_duration_seconds", duration, map[string]string{
		"operation": operation,
	})
}

// RecordKubernetesOperation записывает метрики для Kubernetes операций
func (a *MonitoringAdapter) RecordKubernetesOperation(operation string, resourceType string, status string, duration time.Duration) {
	a.IncCounter("kubernetes_operations_total", map[string]string{
		"operation":     operation,
		"resource_type": resourceType,
		"status":        status,
	})
	a.ObserveDuration("kubernetes_operation_duration_seconds", duration, map[string]string{
		"operation": operation,
	})
}

// RecordCICDOperation записывает метрики для CI/CD операций
func (a *MonitoringAdapter) RecordCICDOperation(operation string, status string, duration time.Duration) {
	a.IncCounter("cicd_operations_total", map[string]string{
		"operation": operation,
		"status":    status,
	})
	a.ObserveDuration("cicd_operation_duration_seconds", duration, map[string]string{
		"operation": operation,
	})
}
