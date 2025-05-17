package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/pkg/errors"
)

// ContainerOptions содержит параметры для создания контейнера
type ContainerOptions struct {
	Image       string
	Name        string
	Ports       map[string]string
	Environment map[string]string
	Volumes     map[string]string
	Network     string
}

// ContainerInfo содержит информацию о контейнере
type ContainerInfo struct {
	ID      string
	Name    string
	Image   string
	Status  string
	Created time.Time
	Ports   map[string]string
	Network string
	State   string
}

// DockerAdapter предоставляет методы для работы с Docker
type DockerAdapter struct {
	client *client.Client
	ctx    context.Context
}

// NewDockerAdapter создает новый экземпляр DockerAdapter
func NewDockerAdapter() (*DockerAdapter, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}

	return &DockerAdapter{
		client: cli,
		ctx:    context.Background(),
	}, nil
}

// PullImage скачивает Docker образ
func (d *DockerAdapter) PullImage(image string) error {
	reader, err := d.client.ImagePull(d.ctx, image, types.ImagePullOptions{})
	if err != nil {
		return errors.Wrap(err, "ошибка при скачивании образа")
	}
	defer reader.Close()

	// Читаем и обрабатываем прогресс скачивания
	decoder := json.NewDecoder(reader)
	for {
		var pullResult struct {
			Status string `json:"status"`
			Error  string `json:"error"`
		}
		if err := decoder.Decode(&pullResult); err != nil {
			if err == io.EOF {
				break
			}
			return errors.Wrap(err, "ошибка при чтении прогресса скачивания")
		}
		if pullResult.Error != "" {
			return errors.New(pullResult.Error)
		}
	}
	return nil
}

// BuildImage собирает Docker образ из Dockerfile
func (d *DockerAdapter) BuildImage(path string, tag string) error {
	buildContext, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return errors.Wrap(err, "ошибка при создании клиента для сборки")
	}

	buildOptions := types.ImageBuildOptions{
		Tags:        []string{tag},
		Remove:      true,
		ForceRemove: true,
	}

	buildResponse, err := buildContext.ImageBuild(d.ctx, nil, buildOptions)
	if err != nil {
		return errors.Wrap(err, "ошибка при сборке образа")
	}
	defer buildResponse.Body.Close()

	// Читаем и обрабатываем прогресс сборки
	decoder := json.NewDecoder(buildResponse.Body)
	for {
		var buildResult struct {
			Stream string `json:"stream"`
			Error  string `json:"error"`
		}
		if err := decoder.Decode(&buildResult); err != nil {
			if err == io.EOF {
				break
			}
			return errors.Wrap(err, "ошибка при чтении прогресса сборки")
		}
		if buildResult.Error != "" {
			return errors.New(buildResult.Error)
		}
	}
	return nil
}

// RunContainer запускает новый контейнер
func (d *DockerAdapter) RunContainer(opts ContainerOptions) (*ContainerInfo, error) {
	// Создаем конфигурацию контейнера
	config := &container.Config{
		Image: opts.Image,
		Env:   make([]string, 0, len(opts.Environment)),
	}

	// Добавляем переменные окружения
	for k, v := range opts.Environment {
		config.Env = append(config.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Создаем хост-конфигурацию
	hostConfig := &container.HostConfig{
		PortBindings: make(map[nat.Port][]nat.PortBinding),
		Binds:        make([]string, 0, len(opts.Volumes)),
	}

	// Настраиваем порты
	for containerPort, hostPort := range opts.Ports {
		port, err := nat.NewPort("tcp", containerPort)
		if err != nil {
			return nil, errors.Wrap(err, "ошибка при парсинге порта контейнера")
		}
		hostConfig.PortBindings[port] = []nat.PortBinding{
			{HostPort: hostPort},
		}
	}

	// Настраиваем тома
	for hostPath, containerPath := range opts.Volumes {
		hostConfig.Binds = append(hostConfig.Binds, fmt.Sprintf("%s:%s", hostPath, containerPath))
	}

	// Создаем контейнер
	resp, err := d.client.ContainerCreate(
		d.ctx,
		config,
		hostConfig,
		&network.NetworkingConfig{},
		nil,
		opts.Name,
	)
	if err != nil {
		return nil, errors.Wrap(err, "ошибка при создании контейнера")
	}

	// Запускаем контейнер
	if err := d.client.ContainerStart(d.ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		// Если не удалось запустить, удаляем контейнер
		_ = d.client.ContainerRemove(d.ctx, resp.ID, types.ContainerRemoveOptions{Force: true})
		return nil, errors.Wrap(err, "ошибка при запуске контейнера")
	}

	// Получаем информацию о контейнере
	container, err := d.client.ContainerInspect(d.ctx, resp.ID)
	if err != nil {
		return nil, errors.Wrap(err, "ошибка при получении информации о контейнере")
	}

	// Парсим время создания
	created, err := strconv.ParseInt(container.Created, 10, 64)
	if err != nil {
		return nil, errors.Wrap(err, "ошибка при парсинге времени создания контейнера")
	}

	return &ContainerInfo{
		ID:      container.ID,
		Name:    container.Name,
		Image:   container.Config.Image,
		Status:  container.State.Status,
		Created: time.Unix(created, 0),
		State:   container.State.Status,
	}, nil
}

// ListContainers возвращает список всех контейнеров
func (d *DockerAdapter) ListContainers() ([]ContainerInfo, error) {
	containers, err := d.client.ContainerList(d.ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return nil, errors.Wrap(err, "ошибка при получении списка контейнеров")
	}

	var result []ContainerInfo
	for _, container := range containers {
		info := ContainerInfo{
			ID:      container.ID,
			Name:    container.Names[0],
			Image:   container.Image,
			Status:  container.Status,
			Created: time.Unix(container.Created, 0),
			State:   container.State,
		}
		result = append(result, info)
	}

	return result, nil
}

// StopContainer останавливает контейнер
func (d *DockerAdapter) StopContainer(containerID string) error {
	timeout := 10 * time.Second
	if err := d.client.ContainerStop(d.ctx, containerID, &timeout); err != nil {
		return errors.Wrap(err, "ошибка при остановке контейнера")
	}
	return nil
}

// RemoveContainer удаляет контейнер
func (d *DockerAdapter) RemoveContainer(containerID string) error {
	if err := d.client.ContainerRemove(d.ctx, containerID, types.ContainerRemoveOptions{
		Force: true,
	}); err != nil {
		return errors.Wrap(err, "ошибка при удалении контейнера")
	}
	return nil
}
