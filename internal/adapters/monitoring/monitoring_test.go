package monitoring

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMonitoringAdapter_RegisterAndIncrementCounters(t *testing.T) {
	adapter := NewMonitoringAdapter(Config{
		Namespace: "test",
		Subsystem: "test",
	})

	// Регистрируем счетчики
	adapter.RegisterCounters(
		[]string{"test_counter_1"},
		[]string{"label1", "label2"},
	)

	// Увеличиваем счетчик
	adapter.IncCounter("test_counter_1", map[string]string{
		"label1": "value1",
		"label2": "value2",
	})

	// Проверяем значение счетчика
	expected := `
		# HELP test_test_test_counter_1 Counter test_counter_1
		# TYPE test_test_test_counter_1 counter
		test_test_test_counter_1{label1="value1",label2="value2"} 1
	`
	err := testutil.GatherAndCompare(prometheus.DefaultGatherer, strings.NewReader(expected), "test_test_test_counter_1")
	require.NoError(t, err)
}

func TestMonitoringAdapter_RegisterAndObserveHistograms(t *testing.T) {
	adapter := NewMonitoringAdapter(Config{
		Namespace: "test",
		Subsystem: "test",
	})

	// Регистрируем гистограммы
	adapter.RegisterHistograms(
		[]string{"test_histogram_1"},
		[]string{"label1", "label2"},
		[]float64{0.1, 0.5, 1.0},
	)

	// Записываем длительность
	adapter.ObserveDuration("test_histogram_1", 500*time.Millisecond, map[string]string{
		"label1": "value1",
		"label2": "value2",
	})

	// Проверяем значение гистограммы
	expected := `
		# HELP test_test_test_histogram_1 Histogram test_histogram_1
		# TYPE test_test_test_histogram_1 histogram
		test_test_test_histogram_1_bucket{label1="value1",label2="value2",le="0.1"} 0
		test_test_test_histogram_1_bucket{label1="value1",label2="value2",le="0.5"} 1
		test_test_test_histogram_1_bucket{label1="value1",label2="value2",le="1.0"} 1
		test_test_test_histogram_1_bucket{label1="value1",label2="value2",le="+Inf"} 1
		test_test_test_histogram_1_sum{label1="value1",label2="value2"} 0.5
		test_test_test_histogram_1_count{label1="value1",label2="value2"} 1
	`
	err := testutil.GatherAndCompare(prometheus.DefaultGatherer, strings.NewReader(expected), "test_test_test_histogram_1")
	require.NoError(t, err)
}

func TestMonitoringAdapter_MetricsHandler(t *testing.T) {
	adapter := NewMonitoringAdapter(Config{
		Namespace: "test",
		Subsystem: "test",
	})

	// Регистрируем тестовые метрики
	adapter.RegisterCounters(
		[]string{"test_counter_2"},
		[]string{"label1"},
	)
	adapter.IncCounter("test_counter_2", map[string]string{"label1": "value1"})

	// Создаем тестовый HTTP сервер
	server := httptest.NewServer(adapter.MetricsHandler())
	defer server.Close()

	// Делаем запрос к эндпоинту метрик
	resp, err := http.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Проверяем статус ответа
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Проверяем заголовки
	assert.Equal(t, "text/plain; version=0.0.4; charset=utf-8", resp.Header.Get("Content-Type"))
}

func TestMonitoringAdapter_UnknownMetrics(t *testing.T) {
	adapter := NewMonitoringAdapter(Config{
		Namespace: "test",
		Subsystem: "test",
	})

	// Пытаемся использовать несуществующие метрики
	adapter.IncCounter("unknown_counter", map[string]string{"label": "value"})
	adapter.ObserveDuration("unknown_histogram", time.Second, map[string]string{"label": "value"})

	// Проверяем, что метрики не были созданы
	expected := ``
	err := testutil.GatherAndCompare(prometheus.DefaultGatherer, strings.NewReader(expected), "test_test_unknown_counter", "test_test_unknown_histogram")
	require.NoError(t, err)
}
