package cicd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// Config содержит конфигурацию для CICD адаптера
type Config struct {
	// BaseURL базовый URL для API CICD системы
	BaseURL string
	// Token токен для аутентификации
	Token string
}

// PipelineStatus представляет статус пайплайна
type PipelineStatus struct {
	ID        string
	Status    string
	StartedAt time.Time
	EndedAt   time.Time
	Duration  time.Duration
	Commit    string
	Branch    string
	Author    string
	Message   string
}

// gitlabPipeline представляет ответ от GitLab API
type gitlabPipeline struct {
	ID        int       `json:"id"`
	Status    string    `json:"status"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"finished_at"`
	Duration  int       `json:"duration"`
	Commit    struct {
		ID      string `json:"id"`
		Message string `json:"message"`
		Author  string `json:"author_name"`
	} `json:"commit"`
	Ref string `json:"ref"`
}

// CICDAdapter предоставляет методы для работы с CICD системой
type CICDAdapter struct {
	config Config
	client *http.Client
}

// NewCICDAdapter создает новый экземпляр CICDAdapter
func NewCICDAdapter(cfg Config) *CICDAdapter {
	return &CICDAdapter{
		config: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// doRequest выполняет HTTP запрос с обработкой ошибок и retry
func (a *CICDAdapter) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	url := fmt.Sprintf("%s/api/v4%s", a.config.BaseURL, path)
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %w", err)
	}

	req.Header.Set("PRIVATE-TOKEN", a.config.Token)
	req.Header.Set("Content-Type", "application/json")

	var resp *http.Response
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		resp, err = a.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("ошибка выполнения запроса: %w", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfter := resp.Header.Get("Retry-After")
			if retryAfter != "" {
				seconds, _ := strconv.Atoi(retryAfter)
				time.Sleep(time.Duration(seconds) * time.Second)
				continue
			}
		}

		if resp.StatusCode >= 400 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("ошибка API (статус %d): %s", resp.StatusCode, string(body))
		}

		return resp, nil
	}

	return nil, fmt.Errorf("превышено максимальное количество попыток")
}

// TriggerPipeline запускает новый пайплайн
func (a *CICDAdapter) TriggerPipeline(ctx context.Context, project string, ref string) (*PipelineStatus, error) {
	path := fmt.Sprintf("/projects/%s/pipeline", project)
	body := map[string]string{
		"ref": ref,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("ошибка сериализации запроса: %w", err)
	}

	resp, err := a.doRequest(ctx, http.MethodPost, path, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var glPipeline gitlabPipeline
	if err := json.NewDecoder(resp.Body).Decode(&glPipeline); err != nil {
		return nil, fmt.Errorf("ошибка разбора ответа: %w", err)
	}

	return &PipelineStatus{
		ID:        strconv.Itoa(glPipeline.ID),
		Status:    glPipeline.Status,
		StartedAt: glPipeline.StartedAt,
		EndedAt:   glPipeline.EndedAt,
		Duration:  time.Duration(glPipeline.Duration) * time.Second,
		Commit:    glPipeline.Commit.ID,
		Branch:    glPipeline.Ref,
		Author:    glPipeline.Commit.Author,
		Message:   glPipeline.Commit.Message,
	}, nil
}

// GetPipelineStatus возвращает статус пайплайна по его ID
func (a *CICDAdapter) GetPipelineStatus(ctx context.Context, project string, pipelineID string) (*PipelineStatus, error) {
	path := fmt.Sprintf("/projects/%s/pipelines/%s", project, pipelineID)
	resp, err := a.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var glPipeline gitlabPipeline
	if err := json.NewDecoder(resp.Body).Decode(&glPipeline); err != nil {
		return nil, fmt.Errorf("ошибка разбора ответа: %w", err)
	}

	return &PipelineStatus{
		ID:        strconv.Itoa(glPipeline.ID),
		Status:    glPipeline.Status,
		StartedAt: glPipeline.StartedAt,
		EndedAt:   glPipeline.EndedAt,
		Duration:  time.Duration(glPipeline.Duration) * time.Second,
		Commit:    glPipeline.Commit.ID,
		Branch:    glPipeline.Ref,
		Author:    glPipeline.Commit.Author,
		Message:   glPipeline.Commit.Message,
	}, nil
}
