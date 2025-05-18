package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/localops/devops-manager/internal/adapters/monitoring"
	"github.com/pkg/errors"
)

// ContainerOptions содержит параметры для создания контейнера
type ContainerOptions struct {
	Image         string
	Name          string
	Ports         map[string]string
	Environment   map[string]string
	Volumes       map[string]string
	Network       string
	Command       []string
	RestartPolicy container.RestartPolicy
	Labels        map[string]string
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
	Labels  map[string]string
}

// ImageInfo содержит информацию об образе
type ImageInfo struct {
	ID       string
	RepoTags []string
	Size     int64
	Created  time.Time
	Labels   map[string]string
}

// DockerAdapter предоставляет методы для работы с Docker
type DockerAdapter struct {
	client     *client.Client
	ctx        context.Context
	registry   *RegistryAdapter
	monitoring *monitoring.MonitoringAdapter
}

// NewDockerAdapter создает новый экземпляр DockerAdapter
func NewDockerAdapter(registryConfig *RegistryConfig, monitoring *monitoring.MonitoringAdapter) (*DockerAdapter, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, errors.Wrap(err, "ошибка при создании Docker клиента")
	}

	adapter := &DockerAdapter{
		client:     cli,
		ctx:        context.Background(),
		monitoring: monitoring,
	}

	if registryConfig != nil {
		adapter.registry = NewRegistryAdapter(*registryConfig)
	}

	return adapter, nil
}

// PullImage скачивает Docker образ
func (d *DockerAdapter) PullImage(image string) error {
	// Создаем команду
	cmd := exec.Command("docker", "pull", image)

	// Перенаправляем вывод
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Запускаем скачивание
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "ошибка при скачивании образа")
	}

	return nil
}

// BuildImage собирает Docker образ
func (d *DockerAdapter) BuildImage(path string, tag string, buildArgs map[string]*string) error {
	start := time.Now()
	err := d.buildImage(path, tag, buildArgs)
	duration := time.Since(start)

	status := "success"
	if err != nil {
		status = "error"
	}

	if d.monitoring != nil {
		d.monitoring.RecordDockerOperation("build_image", status, duration)
	}

	return err
}

// RunContainer создает и запускает контейнер
func (d *DockerAdapter) RunContainer(opts ContainerOptions) (*ContainerInfo, error) {
	start := time.Now()
	container, err := d.runContainer(opts)
	duration := time.Since(start)

	status := "success"
	if err != nil {
		status = "error"
	}

	if d.monitoring != nil {
		d.monitoring.RecordDockerOperation("run_container", status, duration)
	}

	return container, err
}

// ListContainers возвращает список всех контейнеров
func (d *DockerAdapter) ListContainers() ([]ContainerInfo, error) {
	start := time.Now()
	containers, err := d.client.ContainerList(d.ctx, types.ContainerListOptions{All: true})
	duration := time.Since(start)

	status := "success"
	if err != nil {
		status = "error"
	}

	if d.monitoring != nil {
		d.monitoring.RecordDockerOperation("list_containers", status, duration)
	}

	if err != nil {
		return nil, errors.Wrap(err, "ошибка при получении списка контейнеров")
	}

	var result []ContainerInfo
	for _, c := range containers {
		result = append(result, ContainerInfo{
			ID:      c.ID,
			Name:    strings.TrimPrefix(c.Names[0], "/"),
			Image:   c.Image,
			Status:  c.Status,
			Created: time.Unix(c.Created, 0),
		})
	}

	return result, nil
}

// StopContainer останавливает контейнер
func (d *DockerAdapter) StopContainer(containerID string) error {
	start := time.Now()
	err := d.stopContainer(containerID)
	duration := time.Since(start)

	status := "success"
	if err != nil {
		status = "error"
	}

	if d.monitoring != nil {
		d.monitoring.RecordDockerOperation("stop_container", status, duration)
	}

	return err
}

// RemoveContainer удаляет контейнер
func (d *DockerAdapter) RemoveContainer(containerID string) error {
	start := time.Now()
	err := d.client.ContainerRemove(d.ctx, containerID, types.ContainerRemoveOptions{Force: true})
	duration := time.Since(start)

	status := "success"
	if err != nil {
		status = "error"
	}

	if d.monitoring != nil {
		d.monitoring.RecordDockerOperation("remove_container", status, duration)
	}

	if err != nil {
		return errors.Wrap(err, "ошибка при удалении контейнера")
	}

	return nil
}

// ListImages возвращает список всех образов
func (d *DockerAdapter) ListImages() ([]ImageInfo, error) {
	start := time.Now()
	images, err := d.client.ImageList(d.ctx, types.ImageListOptions{})
	duration := time.Since(start)

	status := "success"
	if err != nil {
		status = "error"
	}

	if d.monitoring != nil {
		d.monitoring.RecordDockerOperation("list_images", status, duration)
	}

	if err != nil {
		return nil, errors.Wrap(err, "ошибка при получении списка образов")
	}

	var result []ImageInfo
	for _, img := range images {
		info := ImageInfo{
			ID:       img.ID,
			RepoTags: img.RepoTags,
			Size:     img.Size,
			Created:  time.Unix(img.Created, 0),
			Labels:   img.Labels,
		}
		result = append(result, info)
	}

	return result, nil
}

// RemoveImage удаляет образ
func (d *DockerAdapter) RemoveImage(imageID string) error {
	start := time.Now()
	_, err := d.client.ImageRemove(d.ctx, imageID, types.ImageRemoveOptions{
		Force:         true,
		PruneChildren: true,
	})
	duration := time.Since(start)

	status := "success"
	if err != nil {
		status = "error"
	}

	if d.monitoring != nil {
		d.monitoring.RecordDockerOperation("remove_image", status, duration)
	}

	if err != nil {
		return errors.Wrap(err, "ошибка при удалении образа")
	}
	return nil
}

// GetContainerLogs возвращает логи контейнера
func (d *DockerAdapter) GetContainerLogs(containerID string, since time.Time, tail string) (io.ReadCloser, error) {
	start := time.Now()
	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Since:      since.Format(time.RFC3339),
		Timestamps: true,
	}

	logs, err := d.client.ContainerLogs(d.ctx, containerID, options)
	duration := time.Since(start)

	status := "success"
	if err != nil {
		status = "error"
	}

	if d.monitoring != nil {
		d.monitoring.RecordDockerOperation("get_logs", status, duration)
	}

	if err != nil {
		return nil, errors.Wrap(err, "ошибка при получении логов контейнера")
	}

	return logs, nil
}

// GetContainerStats возвращает статистику контейнера
func (d *DockerAdapter) GetContainerStats(containerID string) (*types.Stats, error) {
	stats, err := d.client.ContainerStats(d.ctx, containerID, false)
	if err != nil {
		return nil, errors.Wrap(err, "ошибка при получении статистики контейнера")
	}
	defer stats.Body.Close()

	var result types.Stats
	if err := json.NewDecoder(stats.Body).Decode(&result); err != nil {
		return nil, errors.Wrap(err, "ошибка при декодировании статистики")
	}

	return &result, nil
}

// CreateNetwork создает новую сеть
func (d *DockerAdapter) CreateNetwork(name string, driver string, options map[string]string) (string, error) {
	start := time.Now()
	resp, err := d.client.NetworkCreate(d.ctx, name, types.NetworkCreate{
		Driver:  driver,
		Options: options,
	})
	duration := time.Since(start)

	status := "success"
	if err != nil {
		status = "error"
	}

	if d.monitoring != nil {
		d.monitoring.RecordDockerOperation("create_network", status, duration)
	}

	if err != nil {
		return "", errors.Wrap(err, "ошибка при создании сети")
	}
	return resp.ID, nil
}

// ConnectContainerToNetwork подключает контейнер к сети
func (d *DockerAdapter) ConnectContainerToNetwork(containerID string, networkID string) error {
	start := time.Now()
	err := d.client.NetworkConnect(d.ctx, networkID, containerID, &network.EndpointSettings{})
	duration := time.Since(start)

	status := "success"
	if err != nil {
		status = "error"
	}

	if d.monitoring != nil {
		d.monitoring.RecordDockerOperation("connect_network", status, duration)
	}

	if err != nil {
		return errors.Wrap(err, "ошибка при подключении контейнера к сети")
	}
	return nil
}

// DisconnectContainerFromNetwork отключает контейнер от сети
func (d *DockerAdapter) DisconnectContainerFromNetwork(containerID string, networkID string) error {
	start := time.Now()
	err := d.client.NetworkDisconnect(d.ctx, networkID, containerID, true)
	duration := time.Since(start)

	status := "success"
	if err != nil {
		status = "error"
	}

	if d.monitoring != nil {
		d.monitoring.RecordDockerOperation("disconnect_network", status, duration)
	}

	if err != nil {
		return errors.Wrap(err, "ошибка при отключении контейнера от сети")
	}
	return nil
}

// PruneSystem очищает неиспользуемые ресурсы
func (d *DockerAdapter) PruneSystem() error {
	start := time.Now()
	_, err := d.client.ContainersPrune(d.ctx, filters.Args{})
	if err != nil {
		return errors.Wrap(err, "ошибка при очистке контейнеров")
	}

	_, err = d.client.ImagesPrune(d.ctx, filters.Args{})
	if err != nil {
		return errors.Wrap(err, "ошибка при очистке образов")
	}

	_, err = d.client.NetworksPrune(d.ctx, filters.Args{})
	if err != nil {
		return errors.Wrap(err, "ошибка при очистке сетей")
	}
	duration := time.Since(start)

	status := "success"
	if err != nil {
		status = "error"
	}

	if d.monitoring != nil {
		d.monitoring.RecordDockerOperation("prune_system", status, duration)
	}

	return nil
}

// Close закрывает соединение с Docker daemon
func (d *DockerAdapter) Close() error {
	return d.client.Close()
}

// PushImageToRegistry отправляет образ в registry
func (d *DockerAdapter) PushImageToRegistry(image string, auth types.AuthConfig) error {
	if d.registry == nil {
		return errors.New("registry не настроен")
	}

	// Отправляем образ в registry
	return d.registry.PushImage(image, auth)
}

// PullImageFromRegistry скачивает образ из registry
func (d *DockerAdapter) PullImageFromRegistry(image string, auth types.AuthConfig) error {
	if d.registry == nil {
		return errors.New("registry не настроен")
	}

	// Скачиваем образ из registry
	return d.registry.PullImage(image, auth)
}

// TagImage создает новый тег для образа
func (d *DockerAdapter) TagImage(sourceImage string, targetImage string) error {
	return d.client.ImageTag(d.ctx, sourceImage, targetImage)
}

// GetImageHistory возвращает историю образа
func (d *DockerAdapter) GetImageHistory(imageID string) ([]image.HistoryResponseItem, error) {
	history, err := d.client.ImageHistory(d.ctx, imageID)
	if err != nil {
		return nil, errors.Wrap(err, "ошибка при получении истории образа")
	}
	return history, nil
}

// GetImageInspect возвращает детальную информацию об образе
func (d *DockerAdapter) GetImageInspect(imageID string) (*types.ImageInspect, error) {
	inspect, _, err := d.client.ImageInspectWithRaw(d.ctx, imageID)
	if err != nil {
		return nil, errors.Wrap(err, "ошибка при получении информации об образе")
	}
	return &inspect, nil
}

// PruneImages удаляет неиспользуемые образы
func (d *DockerAdapter) PruneImages() (*types.ImagesPruneReport, error) {
	report, err := d.client.ImagesPrune(d.ctx, filters.Args{})
	if err != nil {
		return nil, errors.Wrap(err, "ошибка при очистке образов")
	}
	return &report, nil
}

// GetContainerInspect возвращает детальную информацию о контейнере
func (d *DockerAdapter) GetContainerInspect(containerID string) (*types.ContainerJSON, error) {
	inspect, err := d.client.ContainerInspect(d.ctx, containerID)
	if err != nil {
		return nil, errors.Wrap(err, "ошибка при получении информации о контейнере")
	}
	return &inspect, nil
}

// GetContainerProcesses возвращает список процессов в контейнере
func (d *DockerAdapter) GetContainerProcesses(containerID string) ([][]string, error) {
	processes, err := d.client.ContainerTop(d.ctx, containerID, nil)
	if err != nil {
		return nil, errors.Wrap(err, "ошибка при получении списка процессов")
	}
	return processes.Processes, nil
}

// GetContainerChanges возвращает изменения в файловой системе контейнера
func (d *DockerAdapter) GetContainerChanges(containerID string) ([]container.ContainerChangeResponseItem, error) {
	changes, err := d.client.ContainerDiff(d.ctx, containerID)
	if err != nil {
		return nil, errors.Wrap(err, "ошибка при получении изменений в контейнере")
	}
	return changes, nil
}

// PauseContainer приостанавливает контейнер
func (d *DockerAdapter) PauseContainer(containerID string) error {
	return d.client.ContainerPause(d.ctx, containerID)
}

// UnpauseContainer возобновляет работу контейнера
func (d *DockerAdapter) UnpauseContainer(containerID string) error {
	return d.client.ContainerUnpause(d.ctx, containerID)
}

// RestartContainer перезапускает контейнер
func (d *DockerAdapter) RestartContainer(containerID string, timeout *time.Duration) error {
	return d.client.ContainerRestart(d.ctx, containerID, timeout)
}

// RenameContainer переименовывает контейнер
func (d *DockerAdapter) RenameContainer(containerID string, newName string) error {
	return d.client.ContainerRename(d.ctx, containerID, newName)
}

// UpdateContainer обновляет конфигурацию контейнера
func (d *DockerAdapter) UpdateContainer(containerID string, updateConfig container.UpdateConfig) error {
	_, err := d.client.ContainerUpdate(d.ctx, containerID, updateConfig)
	return err
}

// ListNetworks возвращает список всех Docker сетей
func (d *DockerAdapter) ListNetworks() ([]types.NetworkResource, error) {
	networks, err := d.client.NetworkList(d.ctx, types.NetworkListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "ошибка при получении списка сетей")
	}
	return networks, nil
}

// GetSystemInfo возвращает информацию о системе Docker
func (d *DockerAdapter) GetSystemInfo() (*types.Info, error) {
	info, err := d.client.Info(d.ctx)
	if err != nil {
		return nil, errors.Wrap(err, "ошибка при получении системной информации")
	}
	return &info, nil
}

// StartContainer запускает существующий контейнер
func (d *DockerAdapter) StartContainer(containerID string) error {
	start := time.Now()
	err := d.startContainer(containerID)
	duration := time.Since(start)

	status := "success"
	if err != nil {
		status = "error"
	}

	if d.monitoring != nil {
		d.monitoring.RecordDockerOperation("start_container", status, duration)
	}

	return err
}

// GetContainerIDByName возвращает ID контейнера по его имени
func (d *DockerAdapter) GetContainerIDByName(name string) (string, error) {
	containers, err := d.client.ContainerList(d.ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return "", errors.Wrap(err, "ошибка при получении списка контейнеров")
	}

	for _, container := range containers {
		for _, containerName := range container.Names {
			// Docker добавляет '/' в начало имени контейнера
			if strings.TrimPrefix(containerName, "/") == name {
				return container.ID, nil
			}
		}
	}

	return "", errors.New("контейнер с указанным именем не найден")
}

// buildImage собирает Docker образ
func (d *DockerAdapter) buildImage(path string, tag string, buildArgs map[string]*string) error {
	// Формируем команду для сборки
	args := []string{"build", "-t", tag}

	// Добавляем build-аргументы
	for k, v := range buildArgs {
		if v != nil {
			args = append(args, "--build-arg", fmt.Sprintf("%s=%s", k, *v))
		}
	}

	// Добавляем путь к контексту сборки
	args = append(args, path)

	// Создаем команду
	cmd := exec.Command("docker", args...)

	// Перенаправляем вывод
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Запускаем сборку
	return cmd.Run()
}

// runContainer создает и запускает контейнер
func (d *DockerAdapter) runContainer(opts ContainerOptions) (*ContainerInfo, error) {
	// Создаем конфигурацию контейнера
	config := &container.Config{
		Image:  opts.Image,
		Env:    make([]string, 0, len(opts.Environment)),
		Cmd:    opts.Command,
		Labels: opts.Labels,
	}

	// Добавляем переменные окружения
	for k, v := range opts.Environment {
		config.Env = append(config.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Создаем хост-конфигурацию
	hostConfig := &container.HostConfig{
		PortBindings:  make(map[nat.Port][]nat.PortBinding),
		Binds:         make([]string, 0, len(opts.Volumes)),
		RestartPolicy: opts.RestartPolicy,
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
	created, err := time.Parse(time.RFC3339Nano, container.Created)
	if err != nil {
		return nil, errors.Wrap(err, "ошибка при парсинге времени создания контейнера")
	}

	return &ContainerInfo{
		ID:      container.ID,
		Name:    container.Name,
		Image:   container.Config.Image,
		Status:  container.State.Status,
		Created: created,
		State:   container.State.Status,
		Labels:  container.Config.Labels,
	}, nil
}

// startContainer запускает существующий контейнер
func (d *DockerAdapter) startContainer(containerID string) error {
	return d.client.ContainerStart(d.ctx, containerID, types.ContainerStartOptions{})
}

// stopContainer останавливает контейнер
func (d *DockerAdapter) stopContainer(containerID string) error {
	timeout := 10 * time.Second
	return d.client.ContainerStop(d.ctx, containerID, &timeout)
}
