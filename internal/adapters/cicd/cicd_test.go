package cicd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestServer(t *testing.T) (*httptest.Server, *CICDAdapter) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Проверяем заголовки
		assert.Equal(t, "test-token", r.Header.Get("PRIVATE-TOKEN"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		switch r.URL.Path {
		case "/api/v4/projects/test-project/pipeline":
			// Проверяем метод и тело запроса
			assert.Equal(t, http.MethodPost, r.Method)
			var body map[string]string
			err := json.NewDecoder(r.Body).Decode(&body)
			require.NoError(t, err)
			assert.Equal(t, "main", body["ref"])

			// Возвращаем успешный ответ
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(gitlabPipeline{
				ID:        123,
				Status:    "running",
				StartedAt: time.Now(),
				Duration:  0,
				Commit: struct {
					ID      string `json:"id"`
					Message string `json:"message"`
					Author  string `json:"author_name"`
				}{
					ID:      "abc123",
					Message: "test commit",
					Author:  "test author",
				},
				Ref: "main",
			})

		case "/api/v4/projects/test-project/pipelines/123":
			// Проверяем метод
			assert.Equal(t, http.MethodGet, r.Method)

			// Возвращаем успешный ответ
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(gitlabPipeline{
				ID:        123,
				Status:    "success",
				StartedAt: time.Now().Add(-5 * time.Minute),
				EndedAt:   time.Now(),
				Duration:  300,
				Commit: struct {
					ID      string `json:"id"`
					Message string `json:"message"`
					Author  string `json:"author_name"`
				}{
					ID:      "abc123",
					Message: "test commit",
					Author:  "test author",
				},
				Ref: "main",
			})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	adapter := NewCICDAdapter(Config{
		BaseURL: server.URL,
		Token:   "test-token",
	})

	return server, adapter
}

func TestCICDAdapter_TriggerPipeline(t *testing.T) {
	server, adapter := setupTestServer(t)
	defer server.Close()

	ctx := context.Background()
	status, err := adapter.TriggerPipeline(ctx, "test-project", "main")

	require.NoError(t, err)
	assert.NotNil(t, status)
	assert.Equal(t, "123", status.ID)
	assert.Equal(t, "running", status.Status)
	assert.Equal(t, "abc123", status.Commit)
	assert.Equal(t, "main", status.Branch)
	assert.Equal(t, "test author", status.Author)
	assert.Equal(t, "test commit", status.Message)
}

func TestCICDAdapter_GetPipelineStatus(t *testing.T) {
	server, adapter := setupTestServer(t)
	defer server.Close()

	ctx := context.Background()
	status, err := adapter.GetPipelineStatus(ctx, "test-project", "123")

	require.NoError(t, err)
	assert.NotNil(t, status)
	assert.Equal(t, "123", status.ID)
	assert.Equal(t, "success", status.Status)
	assert.Equal(t, "abc123", status.Commit)
	assert.Equal(t, "main", status.Branch)
	assert.Equal(t, "test author", status.Author)
	assert.Equal(t, "test commit", status.Message)
	assert.Equal(t, 5*time.Minute, status.Duration.Round(time.Minute))
}

func TestCICDAdapter_RetryOnRateLimit(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(gitlabPipeline{
			ID:     123,
			Status: "success",
		})
	}))
	defer server.Close()

	adapter := NewCICDAdapter(Config{
		BaseURL: server.URL,
		Token:   "test-token",
	})

	ctx := context.Background()
	status, err := adapter.GetPipelineStatus(ctx, "test-project", "123")

	require.NoError(t, err)
	assert.NotNil(t, status)
	assert.Equal(t, "123", status.ID)
	assert.Equal(t, "success", status.Status)
	assert.Equal(t, 3, attempts)
}

func TestCICDAdapter_ErrorHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "internal server error",
		})
	}))
	defer server.Close()

	adapter := NewCICDAdapter(Config{
		BaseURL: server.URL,
		Token:   "test-token",
	})

	ctx := context.Background()
	status, err := adapter.GetPipelineStatus(ctx, "test-project", "123")

	assert.Error(t, err)
	assert.Nil(t, status)
	assert.Contains(t, err.Error(), "ошибка API (статус 500)")
}
