package docker

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/pkg/errors"
)

// RegistryConfig содержит конфигурацию для работы с Docker Registry
type RegistryConfig struct {
	URL      string
	Username string
	Password string
	Insecure bool
}

// RegistryAdapter предоставляет методы для работы с Docker Registry
type RegistryAdapter struct {
	config RegistryConfig
	client *http.Client
}

// NewRegistryAdapter создает новый экземпляр RegistryAdapter
func NewRegistryAdapter(config RegistryConfig) *RegistryAdapter {
	return &RegistryAdapter{
		config: config,
		client: &http.Client{},
	}
}

// PushImage отправляет образ в registry
func (r *RegistryAdapter) PushImage(image string, auth types.AuthConfig) error {
	// Подготавливаем URL для registry
	registryURL := r.config.URL
	if !strings.HasPrefix(registryURL, "http://") && !strings.HasPrefix(registryURL, "https://") {
		registryURL = "https://" + registryURL
	}

	// Создаем запрос на push
	url := fmt.Sprintf("%s/v2/%s/manifests/latest", registryURL, image)
	req, err := http.NewRequest("PUT", url, nil)
	if err != nil {
		return errors.Wrap(err, "ошибка при создании запроса на push")
	}

	// Добавляем заголовки аутентификации
	authStr := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", auth.Username, auth.Password)))
	req.Header.Set("Authorization", "Basic "+authStr)
	req.Header.Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")

	// Отправляем запрос
	resp, err := r.client.Do(req)
	if err != nil {
		return errors.Wrap(err, "ошибка при отправке запроса на push")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return errors.Errorf("ошибка при push образа: %s, статус: %d", string(body), resp.StatusCode)
	}

	return nil
}

// PullImage скачивает образ из registry
func (r *RegistryAdapter) PullImage(image string, auth types.AuthConfig) error {
	// Подготавливаем URL для registry
	registryURL := r.config.URL
	if !strings.HasPrefix(registryURL, "http://") && !strings.HasPrefix(registryURL, "https://") {
		registryURL = "https://" + registryURL
	}

	// Создаем запрос на pull
	url := fmt.Sprintf("%s/v2/%s/manifests/latest", registryURL, image)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return errors.Wrap(err, "ошибка при создании запроса на pull")
	}

	// Добавляем заголовки аутентификации
	authStr := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", auth.Username, auth.Password)))
	req.Header.Set("Authorization", "Basic "+authStr)
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")

	// Отправляем запрос
	resp, err := r.client.Do(req)
	if err != nil {
		return errors.Wrap(err, "ошибка при отправке запроса на pull")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return errors.Errorf("ошибка при pull образа: %s, статус: %d", string(body), resp.StatusCode)
	}

	return nil
}

// ListTags возвращает список тегов для образа
func (r *RegistryAdapter) ListTags(image string) ([]string, error) {
	// Подготавливаем URL для registry
	registryURL := r.config.URL
	if !strings.HasPrefix(registryURL, "http://") && !strings.HasPrefix(registryURL, "https://") {
		registryURL = "https://" + registryURL
	}

	// Создаем запрос на получение списка тегов
	url := fmt.Sprintf("%s/v2/%s/tags/list", registryURL, image)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "ошибка при создании запроса на получение тегов")
	}

	// Отправляем запрос
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "ошибка при отправке запроса на получение тегов")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, errors.Errorf("ошибка при получении тегов: %s, статус: %d", string(body), resp.StatusCode)
	}

	// Декодируем ответ
	var result struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, errors.Wrap(err, "ошибка при декодировании ответа")
	}

	return result.Tags, nil
}

// DeleteTag удаляет тег из registry
func (r *RegistryAdapter) DeleteTag(image string, tag string, auth types.AuthConfig) error {
	// Подготавливаем URL для registry
	registryURL := r.config.URL
	if !strings.HasPrefix(registryURL, "http://") && !strings.HasPrefix(registryURL, "https://") {
		registryURL = "https://" + registryURL
	}

	// Создаем запрос на удаление тега
	url := fmt.Sprintf("%s/v2/%s/manifests/%s", registryURL, image, tag)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return errors.Wrap(err, "ошибка при создании запроса на удаление тега")
	}

	// Добавляем заголовки аутентификации
	authStr := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", auth.Username, auth.Password)))
	req.Header.Set("Authorization", "Basic "+authStr)

	// Отправляем запрос
	resp, err := r.client.Do(req)
	if err != nil {
		return errors.Wrap(err, "ошибка при отправке запроса на удаление тега")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return errors.Errorf("ошибка при удалении тега: %s, статус: %d", string(body), resp.StatusCode)
	}

	return nil
}

// GetImageDigest возвращает digest образа
func (r *RegistryAdapter) GetImageDigest(image string, tag string, auth types.AuthConfig) (string, error) {
	// Подготавливаем URL для registry
	registryURL := r.config.URL
	if !strings.HasPrefix(registryURL, "http://") && !strings.HasPrefix(registryURL, "https://") {
		registryURL = "https://" + registryURL
	}

	// Создаем запрос на получение digest
	url := fmt.Sprintf("%s/v2/%s/manifests/%s", registryURL, image, tag)
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return "", errors.Wrap(err, "ошибка при создании запроса на получение digest")
	}

	// Добавляем заголовки аутентификации
	authStr := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", auth.Username, auth.Password)))
	req.Header.Set("Authorization", "Basic "+authStr)
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")

	// Отправляем запрос
	resp, err := r.client.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "ошибка при отправке запроса на получение digest")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", errors.Errorf("ошибка при получении digest: %s, статус: %d", string(body), resp.StatusCode)
	}

	// Получаем digest из заголовка
	digest := resp.Header.Get("Docker-Content-Digest")
	if digest == "" {
		return "", errors.New("digest не найден в ответе")
	}

	return digest, nil
}
