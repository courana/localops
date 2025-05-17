package monitoring

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Config содержит конфигурацию для адаптера мониторинга
type Config struct {
	// Namespace для метрик Prometheus
	Namespace string
	// Subsystem для метрик Prometheus
	Subsystem string
}

// MonitoringAdapter предоставляет интерфейс для работы с метриками
type MonitoringAdapter struct {
	config Config
	// Реестр метрик
	registry *prometheus.Registry
	// Счетчики
	counters map[string]*prometheus.CounterVec
	// Гистограммы
	histograms map[string]*prometheus.HistogramVec
}

// NewMonitoringAdapter создает новый экземпляр адаптера мониторинга
func NewMonitoringAdapter(cfg Config) *MonitoringAdapter {
	return &MonitoringAdapter{
		config:     cfg,
		registry:   prometheus.NewRegistry(),
		counters:   make(map[string]*prometheus.CounterVec),
		histograms: make(map[string]*prometheus.HistogramVec),
	}
}

// RegisterCounters регистрирует счетчики с заданными именами и метками
func (a *MonitoringAdapter) RegisterCounters(names []string, labels []string) {
	for _, name := range names {
		a.counters[name] = promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: a.config.Namespace,
				Subsystem: a.config.Subsystem,
				Name:      name,
				Help:      "Counter " + name,
			},
			labels,
		)
	}
}

// RegisterHistograms регистрирует гистограммы с заданными именами и метками
func (a *MonitoringAdapter) RegisterHistograms(names []string, labels []string, buckets []float64) {
	for _, name := range names {
		a.histograms[name] = promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: a.config.Namespace,
				Subsystem: a.config.Subsystem,
				Name:      name,
				Help:      "Histogram " + name,
				Buckets:   buckets,
			},
			labels,
		)
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
