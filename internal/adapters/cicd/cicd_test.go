package cicd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewCICDAdapter(t *testing.T) {
	cfg := Config{
		BaseURL: "http://test.com",
		Token:   "test-token",
	}

	adapter := NewCICDAdapter(cfg)

	if adapter == nil {
		t.Error("NewCICDAdapter вернул nil")
	}

	if adapter.config.BaseURL != cfg.BaseURL {
		t.Errorf("ожидался BaseURL %s, получен %s", cfg.BaseURL, adapter.config.BaseURL)
	}

	if adapter.config.Token != cfg.Token {
		t.Errorf("ожидался Token %s, получен %s", cfg.Token, adapter.config.Token)
	}
}

func TestTriggerPipeline(t *testing.T) {
	// Создаем тестовый сервер
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Проверяем заголовки
		if r.Header.Get("PRIVATE-TOKEN") != "test-token" {
			t.Error("отсутствует или неверный PRIVATE-TOKEN")
		}

		// Проверяем метод
		if r.Method != http.MethodPost {
			t.Errorf("ожидался метод POST, получен %s", r.Method)
		}

		// Отправляем тестовый ответ
		response := gitlabPipeline{
			ID:     123,
			Status: "pending",
			Ref:    "main",
			User: struct {
				Name string `json:"name"`
			}{
				Name: "Test User",
			},
			DetailedStatus: struct {
				Text string `json:"text"`
			}{
				Text: "pending",
			},
			Commit: struct {
				Message string `json:"message"`
				Author  string `json:"author_name"`
			}{
				Message: "Test commit",
				Author:  "Test User",
			},
		}

		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Создаем адаптер с тестовым URL
	adapter := NewCICDAdapter(Config{
		BaseURL: server.URL,
		Token:   "test-token",
	})

	// Тестируем TriggerPipeline
	status, err := adapter.TriggerPipeline(context.Background(), "123", "main")
	if err != nil {
		t.Errorf("TriggerPipeline вернул ошибку: %v", err)
	}

	if status.ID != "123" {
		t.Errorf("ожидался ID 123, получен %s", status.ID)
	}

	if status.Status != "pending" {
		t.Errorf("ожидался статус pending, получен %s", status.Status)
	}
}

func TestGetPipelineStatus(t *testing.T) {
	// Создаем тестовый сервер
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Проверяем метод
		if r.Method != http.MethodGet {
			t.Errorf("ожидался метод GET, получен %s", r.Method)
		}

		// Отправляем тестовый ответ
		now := time.Now()
		response := gitlabPipeline{
			ID:        123,
			Status:    "success",
			Ref:       "main",
			StartedAt: &now,
			EndedAt:   &now,
			Duration:  new(int),
			User: struct {
				Name string `json:"name"`
			}{
				Name: "Test User",
			},
			DetailedStatus: struct {
				Text string `json:"text"`
			}{
				Text: "success",
			},
			Commit: struct {
				Message string `json:"message"`
				Author  string `json:"author_name"`
			}{
				Message: "Test commit",
				Author:  "Test User",
			},
		}

		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Создаем адаптер с тестовым URL
	adapter := NewCICDAdapter(Config{
		BaseURL: server.URL,
		Token:   "test-token",
	})

	// Тестируем GetPipelineStatus
	status, err := adapter.GetPipelineStatus(context.Background(), "123", "456")
	if err != nil {
		t.Errorf("GetPipelineStatus вернул ошибку: %v", err)
	}

	if status.ID != "123" {
		t.Errorf("ожидался ID 123, получен %s", status.ID)
	}

	if status.Status != "success" {
		t.Errorf("ожидался статус success, получен %s", status.Status)
	}
}

func TestListPipelineJobs(t *testing.T) {
	// Создаем тестовый сервер
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Проверяем метод
		if r.Method != http.MethodGet {
			t.Errorf("ожидался метод GET, получен %s", r.Method)
		}

		// Отправляем тестовый ответ
		jobs := []struct {
			ID        int       `json:"id"`
			Name      string    `json:"name"`
			Status    string    `json:"status"`
			Stage     string    `json:"stage"`
			StartedAt time.Time `json:"started_at"`
			EndedAt   time.Time `json:"finished_at"`
			Duration  int       `json:"duration"`
		}{
			{
				ID:        1,
				Name:      "build",
				Status:    "success",
				Stage:     "build",
				StartedAt: time.Now(),
				EndedAt:   time.Now(),
				Duration:  60,
			},
		}

		json.NewEncoder(w).Encode(jobs)
	}))
	defer server.Close()

	// Создаем адаптер с тестовым URL
	adapter := NewCICDAdapter(Config{
		BaseURL: server.URL,
		Token:   "test-token",
	})

	// Тестируем ListPipelineJobs
	jobs, err := adapter.ListPipelineJobs(context.Background(), "123", "456")
	if err != nil {
		t.Errorf("ListPipelineJobs вернул ошибку: %v", err)
	}

	if len(jobs) != 1 {
		t.Errorf("ожидалась 1 задача, получено %d", len(jobs))
	}

	if jobs[0].Name != "build" {
		t.Errorf("ожидалось имя задачи build, получено %s", jobs[0].Name)
	}
}

func TestGetJobLogs(t *testing.T) {
	// Создаем тестовый сервер
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Проверяем метод
		if r.Method != http.MethodGet {
			t.Errorf("ожидался метод GET, получен %s", r.Method)
		}

		// Отправляем тестовые логи
		w.Write([]byte("Test job logs"))
	}))
	defer server.Close()

	// Создаем адаптер с тестовым URL
	adapter := NewCICDAdapter(Config{
		BaseURL: server.URL,
		Token:   "test-token",
	})

	// Тестируем GetJobLogs
	logs, err := adapter.GetJobLogs(context.Background(), "123", "456")
	if err != nil {
		t.Errorf("GetJobLogs вернул ошибку: %v", err)
	}

	if logs != "Test job logs" {
		t.Errorf("ожидались логи 'Test job logs', получено '%s'", logs)
	}
}

func TestCancelPipeline(t *testing.T) {
	// Создаем тестовый сервер
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Проверяем метод
		if r.Method != http.MethodPost {
			t.Errorf("ожидался метод POST, получен %s", r.Method)
		}

		// Отправляем пустой ответ
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Создаем адаптер с тестовым URL
	adapter := NewCICDAdapter(Config{
		BaseURL: server.URL,
		Token:   "test-token",
	})

	// Тестируем CancelPipeline
	err := adapter.CancelPipeline(context.Background(), "123", "456")
	if err != nil {
		t.Errorf("CancelPipeline вернул ошибку: %v", err)
	}
}

func TestRetryPipeline(t *testing.T) {
	// Создаем тестовый сервер
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Проверяем метод
		if r.Method != http.MethodPost {
			t.Errorf("ожидался метод POST, получен %s", r.Method)
		}

		// Отправляем пустой ответ
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Создаем адаптер с тестовым URL
	adapter := NewCICDAdapter(Config{
		BaseURL: server.URL,
		Token:   "test-token",
	})

	// Тестируем RetryPipeline
	err := adapter.RetryPipeline(context.Background(), "123", "456")
	if err != nil {
		t.Errorf("RetryPipeline вернул ошибку: %v", err)
	}
}
