package cicd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
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
	Branch    string
	Author    string
	Message   string
}

// gitlabPipeline представляет ответ от GitLab API
type gitlabPipeline struct {
	ID        int        `json:"id"`
	Status    string     `json:"status"`
	StartedAt *time.Time `json:"started_at"`
	EndedAt   *time.Time `json:"finished_at"`
	Duration  *int       `json:"duration"`
	Ref       string     `json:"ref"`
	User      struct {
		Name string `json:"name"`
	} `json:"user"`
	DetailedStatus struct {
		Text string `json:"text"`
	} `json:"detailed_status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	Commit    struct {
		Message string `json:"message"`
		Author  string `json:"author_name"`
	} `json:"commit"`
}

// PipelineJob содержит информацию о задаче в пайплайне
type PipelineJob struct {
	ID        string
	Name      string
	Status    string
	Stage     string
	StartedAt time.Time
	EndedAt   time.Time
	Duration  time.Duration
}

// CICDAdapter предоставляет методы для работы с CICD системой
type CICDAdapter struct {
	config Config
	client *http.Client
}

// NewCICDAdapter создает новый экземпляр CICDAdapter
func NewCICDAdapter(config Config) *CICDAdapter {
	if config.BaseURL == "" {
		config.BaseURL = "https://gitlab.com" // Устанавливаем значение по умолчанию
	}

	return &CICDAdapter{
		config: config,
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
func (a *CICDAdapter) TriggerPipeline(ctx context.Context, projectID, ref string) (*Pipeline, error) {
	if a.config.Token == "" {
		return nil, fmt.Errorf("токен доступа не установлен")
	}

	url := fmt.Sprintf("%s/api/v4/projects/%s/pipeline", a.config.BaseURL, projectID)
	fmt.Printf("Отправка запроса на URL: %s\n", url)

	// Создаем тело запроса
	body := map[string]string{
		"ref": ref,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("ошибка сериализации запроса: %v", err)
	}
	fmt.Printf("Тело запроса: %s\n", string(jsonBody))

	// Создаем запрос
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %v", err)
	}

	// Добавляем заголовки
	req.Header.Set("PRIVATE-TOKEN", a.config.Token)
	req.Header.Set("Content-Type", "application/json")
	fmt.Printf("Заголовки запроса: %v\n", req.Header)

	// Отправляем запрос
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения запроса: %v", err)
	}
	defer resp.Body.Close()

	// Читаем тело ответа для отладки
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %v", err)
	}
	fmt.Printf("Статус ответа: %d\n", resp.StatusCode)
	fmt.Printf("Тело ответа: %s\n", string(respBody))

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("ошибка API GitLab (статус %d): %s", resp.StatusCode, string(respBody))
	}

	// Парсим ответ
	var glPipeline gitlabPipeline
	if err := json.Unmarshal(respBody, &glPipeline); err != nil {
		return nil, fmt.Errorf("ошибка разбора ответа: %v", err)
	}

	// Создаем объект Pipeline с проверкой на nil
	pipeline := &Pipeline{
		ID:     strconv.Itoa(glPipeline.ID),
		Status: glPipeline.Status,
	}

	// Безопасно обрабатываем время начала
	if glPipeline.StartedAt != nil {
		pipeline.StartedAt = *glPipeline.StartedAt
	} else if glPipeline.CreatedAt != "" {
		if created, err := time.Parse(time.RFC3339, glPipeline.CreatedAt); err == nil {
			pipeline.StartedAt = created
		}
	}

	// Безопасно обрабатываем время окончания
	if glPipeline.EndedAt != nil {
		pipeline.EndedAt = *glPipeline.EndedAt
	}

	// Безопасно обрабатываем длительность
	if glPipeline.Duration != nil {
		pipeline.Duration = time.Duration(*glPipeline.Duration) * time.Second
	} else if !pipeline.StartedAt.IsZero() && !pipeline.EndedAt.IsZero() {
		pipeline.Duration = pipeline.EndedAt.Sub(pipeline.StartedAt)
	}

	// Безопасно обрабатываем автора
	if glPipeline.User.Name != "" {
		pipeline.Author = glPipeline.User.Name
	} else if glPipeline.Commit.Author != "" {
		pipeline.Author = glPipeline.Commit.Author
	}

	// Безопасно обрабатываем сообщение
	if glPipeline.Commit.Message != "" {
		pipeline.Message = glPipeline.Commit.Message
	} else if glPipeline.DetailedStatus.Text != "" {
		pipeline.Message = glPipeline.DetailedStatus.Text
	}

	return pipeline, nil
}

// GetPipelineStatus возвращает статус пайплайна по его ID
func (a *CICDAdapter) GetPipelineStatus(ctx context.Context, project string, pipelineID string) (*PipelineStatus, error) {
	path := fmt.Sprintf("/projects/%s/pipelines/%s", project, pipelineID)
	resp, err := a.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Читаем тело ответа для отладки
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %w", err)
	}

	// Создаем новый Reader для декодирования JSON
	reader := bytes.NewReader(body)
	var glPipeline gitlabPipeline
	if err := json.NewDecoder(reader).Decode(&glPipeline); err != nil {
		return nil, fmt.Errorf("ошибка разбора ответа: %w", err)
	}

	// Проверяем, что все необходимые поля заполнены
	if glPipeline.ID == 0 {
		return nil, fmt.Errorf("неверный ID пайплайна")
	}

	// Создаем базовый статус
	status := &PipelineStatus{
		ID:      strconv.Itoa(glPipeline.ID),
		Status:  glPipeline.Status,
		Branch:  glPipeline.Ref,
		Author:  glPipeline.User.Name,
		Message: glPipeline.Commit.Message,
	}

	// Обработка времени начала
	if glPipeline.StartedAt != nil {
		status.StartedAt = *glPipeline.StartedAt
	} else if glPipeline.CreatedAt != "" {
		if created, err := time.Parse(time.RFC3339, glPipeline.CreatedAt); err == nil {
			status.StartedAt = created
		}
	}

	// Обработка времени окончания
	if glPipeline.EndedAt != nil {
		status.EndedAt = *glPipeline.EndedAt
	}

	// Обработка длительности
	if glPipeline.Duration != nil {
		status.Duration = time.Duration(*glPipeline.Duration) * time.Second
	} else if !status.StartedAt.IsZero() && !status.EndedAt.IsZero() {
		status.Duration = status.EndedAt.Sub(status.StartedAt)
	}

	// Если автор не указан, используем имя из коммита
	if status.Author == "" && glPipeline.Commit.Author != "" {
		status.Author = glPipeline.Commit.Author
	}

	// Если сообщение не указано, используем текст из detailed_status
	if status.Message == "" && glPipeline.DetailedStatus.Text != "" {
		status.Message = glPipeline.DetailedStatus.Text
	}

	return status, nil
}

// ListPipelineJobs возвращает список задач в пайплайне
func (c *CICDAdapter) ListPipelineJobs(ctx context.Context, projectID, pipelineID string) ([]PipelineJob, error) {
	path := fmt.Sprintf("/projects/%s/pipelines/%s/jobs", projectID, pipelineID)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var jobs []struct {
		ID        int       `json:"id"`
		Name      string    `json:"name"`
		Status    string    `json:"status"`
		Stage     string    `json:"stage"`
		StartedAt time.Time `json:"started_at"`
		EndedAt   time.Time `json:"finished_at"`
		Duration  int       `json:"duration"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
		return nil, fmt.Errorf("ошибка разбора ответа: %w", err)
	}

	var result []PipelineJob
	for _, job := range jobs {
		result = append(result, PipelineJob{
			ID:        strconv.Itoa(job.ID),
			Name:      job.Name,
			Status:    job.Status,
			Stage:     job.Stage,
			StartedAt: job.StartedAt,
			EndedAt:   job.EndedAt,
			Duration:  time.Duration(job.Duration) * time.Second,
		})
	}

	return result, nil
}

// GetJobLogs возвращает логи задачи
func (c *CICDAdapter) GetJobLogs(ctx context.Context, projectID, jobID string) (string, error) {
	path := fmt.Sprintf("/projects/%s/jobs/%s/trace", projectID, jobID)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	logs, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ошибка чтения логов: %w", err)
	}

	return string(logs), nil
}

// CancelPipeline отменяет выполняющийся пайплайн
func (c *CICDAdapter) CancelPipeline(ctx context.Context, projectID, pipelineID string) error {
	path := fmt.Sprintf("/projects/%s/pipelines/%s/cancel", projectID, pipelineID)
	resp, err := c.doRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// RetryPipeline перезапускает упавший пайплайн
func (c *CICDAdapter) RetryPipeline(ctx context.Context, projectID, pipelineID string) error {
	path := fmt.Sprintf("/projects/%s/pipelines/%s/retry", projectID, pipelineID)
	resp, err := c.doRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// DownloadArtifacts скачивает артефакты сборки
func (c *CICDAdapter) DownloadArtifacts(ctx context.Context, projectID, jobID, outputPath string) error {
	path := fmt.Sprintf("/projects/%s/jobs/%s/artifacts", projectID, jobID)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Создаем директорию если не существует
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("ошибка при создании директории: %w", err)
	}

	// Создаем файл для сохранения артефактов
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("ошибка при создании файла: %w", err)
	}
	defer file.Close()

	// Копируем данные из ответа в файл
	if _, err := io.Copy(file, resp.Body); err != nil {
		return fmt.Errorf("ошибка при сохранении артефактов: %w", err)
	}

	return nil
}

// Pipeline содержит информацию о пайплайне
type Pipeline struct {
	ID        string
	Status    string
	StartedAt time.Time
	EndedAt   time.Time
	Duration  time.Duration
	Author    string
	Message   string
}

// CreateOrUpdateGitLabCI создает или обновляет файл .gitlab-ci.yml
func (c *CICDAdapter) CreateOrUpdateGitLabCI(name string, data map[string]string) error {
	// Формируем базовую конфигурацию
	config := `stages:
  - build
  - test
  - deploy

variables:
  DOCKER_IMAGE: ${CI_REGISTRY_IMAGE}:${CI_COMMIT_REF_SLUG}

build:
  stage: build
  image: golang:1.21
  script:
    - go mod download
    - go build -o app ./cmd/cli
  artifacts:
    paths:
      - app

test:
  stage: test
  image: golang:1.21
  script:
    - go test ./...

deploy:
  stage: deploy
  image: docker:latest
  services:
    - docker:dind
  script:
    - docker build -t $DOCKER_IMAGE .
    - docker push $DOCKER_IMAGE
  only:
    - main
`

	// Добавляем пользовательские переменные
	if len(data) > 0 {
		config += "\nvariables:\n"
		for key, value := range data {
			config += fmt.Sprintf("  %s: %s\n", key, value)
		}
	}

	// Создаем файл
	err := os.WriteFile(".gitlab-ci.yml", []byte(config), 0644)
	if err != nil {
		return fmt.Errorf("ошибка при создании файла: %w", err)
	}

	return nil
}

// GetGitLabCI возвращает содержимое файла .gitlab-ci.yml
func (c *CICDAdapter) GetGitLabCI() (string, error) {
	content, err := os.ReadFile(".gitlab-ci.yml")
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("файл .gitlab-ci.yml не найден")
		}
		return "", fmt.Errorf("ошибка при чтении файла: %w", err)
	}

	return string(content), nil
}
