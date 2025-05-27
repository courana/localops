package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/localops/devops-manager/internal/adapters/cicd"
	"github.com/localops/devops-manager/internal/adapters/docker"
	"github.com/localops/devops-manager/internal/adapters/kubernetes"
	"github.com/localops/devops-manager/internal/adapters/monitoring"
)

type Menu struct {
	dockerAdapter     *docker.DockerAdapter
	k8sAdapter        *kubernetes.K8sAdapter
	cicdAdapter       *cicd.CICDAdapter
	monitoringAdapter *monitoring.MonitoringAdapter
	scanner           *bufio.Scanner
}

func NewMenu() (*Menu, error) {
	// Инициализация Docker Registry конфигурации
	registryConfig := &docker.RegistryConfig{
		URL:      os.Getenv("DOCKER_REGISTRY_URL"),
		Username: os.Getenv("DOCKER_REGISTRY_USERNAME"),
		Password: os.Getenv("DOCKER_REGISTRY_PASSWORD"),
		Insecure: os.Getenv("DOCKER_REGISTRY_INSECURE") == "true",
	}

	// Инициализация Monitoring адаптера
	monitoringAdapter := monitoring.NewMonitoringAdapter(monitoring.Config{
		Namespace: "devops",
		Subsystem: "manager",
		Port:      9090,
	})

	// Инициализация Docker адаптера
	dockerAdapter, err := docker.NewDockerAdapter(registryConfig, monitoringAdapter)
	if err != nil {
		return nil, fmt.Errorf("ошибка при инициализации Docker адаптера: %v", err)
	}

	// Инициализация Kubernetes адаптера
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении домашней директории: %v", err)
	}
	kubeconfigPath := filepath.Join(homeDir, ".kube", "config")
	k8sAdapter, err := kubernetes.NewK8sAdapter(kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("ошибка при инициализации Kubernetes адаптера: %v", err)
	}

	// Инициализация CI/CD адаптера с проверкой переменных окружения
	cicdBaseURL := os.Getenv("CICD_BASE_URL")
	if cicdBaseURL == "" {
		cicdBaseURL = "https://gitlab.com" // Значение по умолчанию
	}

	cicdToken := os.Getenv("CICD_TOKEN")
	if cicdToken == "" {
		fmt.Println("Предупреждение: CICD_TOKEN не установлен. CI/CD функции будут недоступны.")
	}

	cicdAdapter := cicd.NewCICDAdapter(cicd.Config{
		BaseURL: cicdBaseURL,
		Token:   cicdToken,
	})

	return &Menu{
		dockerAdapter:     dockerAdapter,
		k8sAdapter:        k8sAdapter,
		cicdAdapter:       cicdAdapter,
		monitoringAdapter: monitoringAdapter,
		scanner:           bufio.NewScanner(os.Stdin),
	}, nil
}

func (m *Menu) readInput() string {
	m.scanner.Scan()
	return strings.TrimSpace(m.scanner.Text())
}

func (m *Menu) printMainMenu() {
	fmt.Println("\n=== DevOps Manager CLI ===")
	fmt.Println("1. Управление Docker-образами")
	fmt.Println("2. Управление контейнерами")
	fmt.Println("3. Управление Kubernetes")
	fmt.Println("4. Управление CI/CD")
	fmt.Println("5. Мониторинг")
	fmt.Println("0. Выход")
	fmt.Print("Выберите пункт меню: ")
}

func (m *Menu) printImageMenu() {
	fmt.Println("\n=== Управление Docker-образами ===")
	fmt.Println("1. Собрать образ")
	fmt.Println("2. Список образов")
	fmt.Println("3. Удалить образ")
	fmt.Println("4. Информация об образе")
	fmt.Println("0. Назад")
	fmt.Print("Выберите пункт меню: ")
}

func (m *Menu) printContainerMenu() {
	fmt.Println("\n=== Управление контейнерами ===")
	fmt.Println("1. Создать контейнер")
	fmt.Println("2. Список контейнеров")
	fmt.Println("3. Запустить контейнер")
	fmt.Println("4. Остановить контейнер")
	fmt.Println("5. Удалить контейнер")
	fmt.Println("6. Логи контейнера")
	fmt.Println("7. Перезапустить контейнер")
	fmt.Println("0. Назад")
	fmt.Print("Выберите пункт меню: ")
}

func (m *Menu) printNetworkMenu() {
	fmt.Println("\n=== Управление сетями ===")
	fmt.Println("1. Создать сеть")
	fmt.Println("2. Список сетей")
	fmt.Println("3. Подключить контейнер к сети")
	fmt.Println("4. Отключить контейнер от сети")
	fmt.Println("0. Назад")
	fmt.Print("Выберите пункт меню: ")
}

func (m *Menu) printMaintenanceMenu() {
	fmt.Println("\n=== Системное обслуживание ===")
	fmt.Println("1. Очистка неиспользуемых ресурсов")
	fmt.Println("2. Системная информация")
	fmt.Println("0. Назад")
	fmt.Print("Выберите пункт меню: ")
}

func (m *Menu) printKubernetesMenu() {
	fmt.Println("\n=== Управление Kubernetes ===")
	fmt.Println("1. Применить манифест")
	fmt.Println("2. Масштабировать деплоймент")
	fmt.Println("3. Статус подов")
	fmt.Println("4. Статус деплоймента")
	fmt.Println("5. Список сервисов и маршрутов")
	fmt.Println("6. Удалить ресурс")
	fmt.Println("7. Управление конфигурацией")
	fmt.Println("8. Управление секретами")
	fmt.Println("0. Назад")
	fmt.Print("Выберите пункт меню: ")
}

func (m *Menu) printCICDMenu() {
	fmt.Println("\n=== Управление CI/CD ===")
	fmt.Println("1. Запустить сборку")
	fmt.Println("2. Статус сборки")
	fmt.Println("3. Список задач")
	fmt.Println("4. Логи задачи")
	fmt.Println("5. Отменить сборку")
	fmt.Println("6. Перезапустить сборку")
	fmt.Println("7. Скачать артефакты")
	fmt.Println("8. Создать/настроить .gitlab-ci.yml")
	fmt.Println("0. Назад")
	fmt.Print("Выберите пункт меню: ")
}

func (m *Menu) printMonitoringMenu() {
	fmt.Println("\n=== Мониторинг ===")
	fmt.Println("1. Сырые метрики")
	fmt.Println("2. Запрос метрики")
	fmt.Println("3. Список метрик")
	fmt.Println("4. Проверка здоровья")
	fmt.Println("0. Назад")
	fmt.Print("Выберите пункт меню: ")
}

func (m *Menu) printConfigMenu() {
	fmt.Println("\n=== Управление конфигурацией ===")
	fmt.Println("1. Создать/обновить ConfigMap")
	fmt.Println("2. Просмотреть ConfigMap")
	fmt.Println("3. Настроить конфигурацию nginx")
	fmt.Println("4. Список всех ConfigMap")
	fmt.Println("0. Назад")
	fmt.Print("Выберите действие: ")
}

func (m *Menu) printSecretMenu() {
	fmt.Println("\n=== Управление секретами ===")
	fmt.Println("1. Создать/обновить секрет")
	fmt.Println("2. Просмотреть секрет")
	fmt.Println("3. Список всех секретов")
	fmt.Println("0. Назад")
	fmt.Print("Выберите действие: ")
}

func (m *Menu) handleImageMenu() {
	for {
		m.printImageMenu()
		choice := m.readInput()

		switch choice {
		case "1":
			m.buildImage()
		case "2":
			m.listImages()
		case "3":
			m.removeImage()
		case "4":
			m.inspectImage()
		case "0":
			return
		default:
			fmt.Println("Неверный выбор")
		}
	}
}

func (m *Menu) handleContainerMenu() {
	for {
		m.printContainerMenu()
		choice := m.readInput()

		switch choice {
		case "1":
			m.createContainer()
		case "2":
			m.listContainers()
		case "3":
			m.startContainer()
		case "4":
			m.stopContainer()
		case "5":
			m.removeContainer()
		case "6":
			m.containerLogs()
		case "7":
			m.restartContainer()
		case "0":
			return
		default:
			fmt.Println("Неверный выбор")
		}
	}
}

func (m *Menu) handleNetworkMenu() {
	for {
		m.printNetworkMenu()
		choice := m.readInput()

		switch choice {
		case "1":
			m.createNetwork()
		case "2":
			m.listNetworks()
		case "3":
			m.connectContainerToNetwork()
		case "4":
			m.disconnectContainerFromNetwork()
		case "0":
			return
		default:
			fmt.Println("Неверный выбор")
		}
	}
}

func (m *Menu) handleMaintenanceMenu() {
	for {
		m.printMaintenanceMenu()
		choice := m.readInput()

		switch choice {
		case "1":
			m.pruneSystem()
		case "2":
			m.systemInfo()
		case "0":
			return
		default:
			fmt.Println("Неверный выбор")
		}
	}
}

func (m *Menu) handleKubernetesMenu() {
	for {
		m.printKubernetesMenu()
		choice := m.readInput()

		switch choice {
		case "1":
			m.deployManifest()
		case "2":
			m.scaleDeployment()
		case "3":
			m.getPodStatuses()
		case "4":
			m.getDeploymentStatus()
		case "5":
			m.listServicesAndIngresses()
		case "6":
			m.deleteResource()
		case "7":
			m.handleConfigMenu()
		case "8":
			m.handleSecretMenu()
		case "0":
			return
		default:
			fmt.Println("Неверный выбор")
		}
	}
}

func (m *Menu) handleCICDMenu() {
	for {
		m.printCICDMenu()
		choice := m.readInput()

		switch choice {
		case "1":
			m.triggerPipeline()
		case "2":
			m.getPipelineStatus()
		case "3":
			m.listPipelineJobs()
		case "4":
			m.viewJobLogs()
		case "5":
			m.cancelPipeline()
		case "6":
			m.retryPipeline()
		case "7":
			m.downloadArtifacts()
		case "8":
			m.configureGitLabCI()
		case "0":
			return
		default:
			fmt.Println("Неверный выбор")
		}
	}
}

func (m *Menu) handleMonitoringMenu() {
	for {
		m.printMonitoringMenu()
		choice := m.readInput()

		switch choice {
		case "1":
			m.showRawMetrics()
		case "2":
			m.queryMetric()
		case "3":
			m.listMetrics()
		case "4":
			m.showServiceHealth()
		case "0":
			return
		default:
			fmt.Println("Неверный выбор")
		}
	}
}

func (m *Menu) handleConfigMenu() {
	for {
		m.printConfigMenu()
		choice := m.readInput()

		switch choice {
		case "1":
			m.createOrUpdateConfigMap()
		case "2":
			m.viewConfigMap()
		case "3":
			m.configureNginx()
		case "4":
			m.listConfigMaps()
		case "0":
			return
		default:
			fmt.Println("Неверный выбор")
		}
	}
}

func (m *Menu) handleSecretMenu() {
	for {
		m.printSecretMenu()
		choice := m.readInput()

		switch choice {
		case "1":
			m.createOrUpdateSecret()
		case "2":
			m.viewSecret()
		case "3":
			m.listSecrets()
		case "0":
			return
		default:
			fmt.Println("Неверный выбор")
		}
	}
}

func (m *Menu) buildImage() {
	fmt.Print("Введите путь к директории с Dockerfile: ")
	path := m.readInput()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Printf("Ошибка: директория %s не существует\n", path)
		return
	}

	dockerfilePath := filepath.Join(path, "Dockerfile")
	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
		fmt.Printf("Ошибка: Dockerfile не найден в директории %s\n", path)
		fmt.Println("Убедитесь, что файл Dockerfile существует в указанной директории")
		return
	}

	fmt.Print("Введите тег образа (например, calculator:latest): ")
	tag := m.readInput()

	buildArgs := make(map[string]*string)
	fmt.Print("Введите build-аргументы (формат: KEY=VALUE, пустая строка для завершения): ")
	for {
		arg := m.readInput()
		if arg == "" {
			break
		}
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) == 2 {
			buildArgs[parts[0]] = &parts[1]
		}
	}

	fmt.Printf("Начинаем сборку образа %s из директории %s...\n", tag, path)
	err := m.dockerAdapter.BuildImage(path, tag, buildArgs)
	if err != nil {
		fmt.Printf("Ошибка при сборке образа: %v\n", err)
		return
	}
	fmt.Println("Образ успешно собран")
}

func (m *Menu) listImages() {
	images, err := m.dockerAdapter.ListImages()
	if err != nil {
		fmt.Printf("Ошибка при получении списка образов: %v\n", err)
		return
	}

	// Сортировка образов по дате создания в обратном порядке
	sort.Slice(images, func(i, j int) bool {
		return images[i].Created.After(images[j].Created)
	})

	fmt.Println("\nСписок образов:")
	for _, img := range images {
		fmt.Printf("ID: %s\n", img.ID)
		fmt.Printf("Теги: %v\n", img.RepoTags)
		fmt.Printf("Размер: %d байт\n", img.Size)
		fmt.Printf("Создан: %s\n", img.Created)
		fmt.Println("---")
	}
}

func (m *Menu) removeImage() {
	fmt.Print("Введите имя образа (например, myapp:latest): ")
	imageName := m.readInput()

	err := m.dockerAdapter.RemoveImage(imageName)
	if err != nil {
		fmt.Printf("Ошибка при удалении образа: %v\n", err)
		return
	}
	fmt.Println("Образ успешно удален")
}

func (m *Menu) inspectImage() {
	fmt.Print("Введите имя образа (например, myapp:latest): ")
	imageName := m.readInput()

	inspect, err := m.dockerAdapter.GetImageInspect(imageName)
	if err != nil {
		fmt.Printf("Ошибка при получении информации об образе: %v\n", err)
		return
	}

	jsonData, err := json.MarshalIndent(inspect, "", "  ")
	if err != nil {
		fmt.Printf("Ошибка при форматировании информации: %v\n", err)
		return
	}
	fmt.Printf("\nИнформация об образе:\n%s\n", string(jsonData))
}

func (m *Menu) createContainer() {
	fmt.Print("Введите имя образа: ")
	image := m.readInput()
	fmt.Print("Введите имя контейнера: ")
	name := m.readInput()

	ports := make(map[string]string)
	fmt.Print("Введите маппинг портов (формат: containerPort:hostPort, пустая строка для завершения): ")
	for {
		port := m.readInput()
		if port == "" {
			break
		}
		parts := strings.Split(port, ":")
		if len(parts) == 2 {
			ports[parts[0]] = parts[1]
		}
	}

	env := make(map[string]string)
	fmt.Print("Введите переменные окружения (формат: KEY=VALUE, пустая строка для завершения): ")
	for {
		envVar := m.readInput()
		if envVar == "" {
			break
		}
		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}

	opts := docker.ContainerOptions{
		Image:       image,
		Name:        name,
		Ports:       ports,
		Environment: env,
		RestartPolicy: container.RestartPolicy{
			Name: "always",
		},
	}

	container, err := m.dockerAdapter.RunContainer(opts)
	if err != nil {
		fmt.Printf("Ошибка при создании контейнера: %v\n", err)
		return
	}
	fmt.Printf("Контейнер успешно создан. ID: %s\n", container.ID)
}

func (m *Menu) startContainer() {
	fmt.Print("Введите имя контейнера: ")
	containerName := m.readInput()

	containerID, err := m.dockerAdapter.GetContainerIDByName(containerName)
	if err != nil {
		fmt.Printf("Ошибка: %v\n", err)
		return
	}

	err = m.dockerAdapter.StartContainer(containerID)
	if err != nil {
		fmt.Printf("Ошибка при запуске контейнера: %v\n", err)
		return
	}
	fmt.Println("Контейнер успешно запущен")
}

func (m *Menu) listContainers() {
	containers, err := m.dockerAdapter.ListContainers()
	if err != nil {
		fmt.Printf("Ошибка при получении списка контейнеров: %v\n", err)
		return
	}

	fmt.Println("\nСписок контейнеров:")
	for _, c := range containers {
		fmt.Printf("ID: %s\n", c.ID)
		fmt.Printf("Имя: %s\n", c.Name)
		fmt.Printf("Образ: %s\n", c.Image)
		fmt.Printf("Статус: %s\n", c.Status)
		fmt.Printf("Создан: %s\n", c.Created)
		fmt.Println("---")
	}
}

func (m *Menu) stopContainer() {
	fmt.Print("Введите имя контейнера: ")
	containerName := m.readInput()

	containerID, err := m.dockerAdapter.GetContainerIDByName(containerName)
	if err != nil {
		fmt.Printf("Ошибка: %v\n", err)
		return
	}

	err = m.dockerAdapter.StopContainer(containerID)
	if err != nil {
		fmt.Printf("Ошибка при остановке контейнера: %v\n", err)
		return
	}
	fmt.Println("Контейнер успешно остановлен")
}

func (m *Menu) removeContainer() {
	fmt.Print("Введите имя контейнера: ")
	containerName := m.readInput()

	containerID, err := m.dockerAdapter.GetContainerIDByName(containerName)
	if err != nil {
		fmt.Printf("Ошибка: %v\n", err)
		return
	}

	err = m.dockerAdapter.RemoveContainer(containerID)
	if err != nil {
		fmt.Printf("Ошибка при удалении контейнера: %v\n", err)
		return
	}
	fmt.Println("Контейнер успешно удален")
}

func (m *Menu) containerLogs() {
	fmt.Print("Введите имя контейнера: ")
	containerName := m.readInput()
	fmt.Print("Введите количество последних строк (или 'all'): ")
	tail := m.readInput()

	containerID, err := m.dockerAdapter.GetContainerIDByName(containerName)
	if err != nil {
		fmt.Printf("Ошибка: %v\n", err)
		return
	}

	logs, err := m.dockerAdapter.GetContainerLogs(containerID, time.Time{}, tail)
	if err != nil {
		fmt.Printf("Ошибка при получении логов: %v\n", err)
		return
	}
	defer logs.Close()

	scanner := bufio.NewScanner(logs)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}
}

func (m *Menu) containerStats() {
	fmt.Print("Введите ID контейнера: ")
	containerID := m.readInput()

	stats, err := m.dockerAdapter.GetContainerStats(containerID)
	if err != nil {
		fmt.Printf("Ошибка при получении статистики: %v\n", err)
		return
	}

	jsonData, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		fmt.Printf("Ошибка при форматировании статистики: %v\n", err)
		return
	}
	fmt.Printf("\nСтатистика контейнера:\n%s\n", string(jsonData))
}

func (m *Menu) containerProcesses() {
	fmt.Print("Введите ID контейнера: ")
	containerID := m.readInput()

	processes, err := m.dockerAdapter.GetContainerProcesses(containerID)
	if err != nil {
		fmt.Printf("Ошибка при получении списка процессов: %v\n", err)
		return
	}

	fmt.Println("\nПроцессы в контейнере:")
	for _, proc := range processes {
		fmt.Println(strings.Join(proc, "\t"))
	}
}

func (m *Menu) createNetwork() {
	fmt.Print("Введите имя сети: ")
	name := m.readInput()
	fmt.Print("Введите драйвер сети (bridge/host/none): ")
	driver := m.readInput()

	options := make(map[string]string)
	fmt.Print("Введите опции сети (формат: KEY=VALUE, пустая строка для завершения): ")
	for {
		opt := m.readInput()
		if opt == "" {
			break
		}
		parts := strings.SplitN(opt, "=", 2)
		if len(parts) == 2 {
			options[parts[0]] = parts[1]
		}
	}

	networkID, err := m.dockerAdapter.CreateNetwork(name, driver, options)
	if err != nil {
		fmt.Printf("Ошибка при создании сети: %v\n", err)
		return
	}
	fmt.Printf("Сеть успешно создана. ID: %s\n", networkID)
}

func (m *Menu) listNetworks() {
	networks, err := m.dockerAdapter.ListNetworks()
	if err != nil {
		fmt.Printf("Ошибка при получении списка сетей: %v\n", err)
		return
	}

	fmt.Println("\nСписок сетей:")
	for _, network := range networks {
		fmt.Printf("ID: %s\n", network.ID)
		fmt.Printf("Имя: %s\n", network.Name)
		fmt.Printf("Драйвер: %s\n", network.Driver)
		fmt.Printf("Область: %s\n", network.Scope)
		fmt.Println("---")
	}
}

func (m *Menu) connectContainerToNetwork() {
	fmt.Print("Введите ID контейнера: ")
	containerID := m.readInput()
	fmt.Print("Введите ID сети: ")
	networkID := m.readInput()

	err := m.dockerAdapter.ConnectContainerToNetwork(containerID, networkID)
	if err != nil {
		fmt.Printf("Ошибка при подключении контейнера к сети: %v\n", err)
		return
	}
	fmt.Println("Контейнер успешно подключен к сети")
}

func (m *Menu) disconnectContainerFromNetwork() {
	fmt.Print("Введите ID контейнера: ")
	containerID := m.readInput()
	fmt.Print("Введите ID сети: ")
	networkID := m.readInput()

	err := m.dockerAdapter.DisconnectContainerFromNetwork(containerID, networkID)
	if err != nil {
		fmt.Printf("Ошибка при отключении контейнера от сети: %v\n", err)
		return
	}
	fmt.Println("Контейнер успешно отключен от сети")
}

func (m *Menu) pruneSystem() {
	err := m.dockerAdapter.PruneSystem()
	if err != nil {
		fmt.Printf("Ошибка при очистке системы: %v\n", err)
		return
	}
	fmt.Println("Система успешно очищена")
}

func (m *Menu) systemInfo() {
	info, err := m.dockerAdapter.GetSystemInfo()
	if err != nil {
		fmt.Printf("Ошибка при получении системной информации: %v\n", err)
		return
	}

	jsonData, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		fmt.Printf("Ошибка при форматировании информации: %v\n", err)
		return
	}
	fmt.Printf("\nСистемная информация:\n%s\n", string(jsonData))
}

// Kubernetes методы
func (m *Menu) deployManifest() {
	fmt.Print("Введите путь к YAML файлу манифеста: ")
	manifestPath := m.readInput()

	err := m.k8sAdapter.ApplyManifest(manifestPath)
	if err != nil {
		fmt.Printf("Ошибка при применении манифеста: %v\n", err)
		return
	}
	fmt.Println("Манифест успешно применен")
}

func (m *Menu) scaleDeployment() {
	fmt.Print("Введите имя деплоймента: ")
	name := m.readInput()
	fmt.Print("Введите новое количество реплик: ")
	replicas := m.readInput()

	replicasInt, err := strconv.Atoi(replicas)
	if err != nil {
		fmt.Println("Ошибка: введите корректное число реплик")
		return
	}

	err = m.k8sAdapter.Scale("default", name, int32(replicasInt))
	if err != nil {
		fmt.Printf("Ошибка при масштабировании деплоймента: %v\n", err)
		return
	}
	fmt.Println("Деплоймент успешно масштабирован")
}

func (m *Menu) getPodStatuses() {
	pods, err := m.k8sAdapter.GetPodStatuses("default")
	if err != nil {
		fmt.Printf("Ошибка при получении списка подов: %v\n", err)
		return
	}

	fmt.Println("\nСписок подов:")
	for _, pod := range pods {
		fmt.Printf("Имя: %s\n", pod.Name)
		fmt.Printf("Статус: %s\n", pod.Status)
		fmt.Printf("IP: %s\n", pod.IP)
		fmt.Printf("Готов: %v\n", pod.Ready)
		fmt.Printf("Рестарты: %d\n", pod.Restarts)
		fmt.Println("---")
	}
}

func (m *Menu) getDeploymentStatus() {
	fmt.Print("Введите имя деплоймента: ")
	name := m.readInput()

	status, err := m.k8sAdapter.GetDeploymentStatus("default", name)
	if err != nil {
		fmt.Printf("Ошибка при получении статуса деплоймента: %v\n", err)
		return
	}

	fmt.Printf("\nСтатус деплоймента %s:\n", status.Name)
	fmt.Printf("Namespace: %s\n", status.Namespace)
	fmt.Printf("Желаемое количество реплик: %d\n", status.Replicas)
	fmt.Printf("Готовых реплик: %d\n", status.ReadyReplicas)
	fmt.Printf("Обновленных реплик: %d\n", status.UpdatedReplicas)
	fmt.Printf("Доступных реплик: %d\n", status.AvailableReplicas)
	fmt.Printf("Недоступных реплик: %d\n", status.UnavailableReplicas)

	if len(status.Conditions) > 0 {
		fmt.Println("\nУсловия:")
		for _, condition := range status.Conditions {
			fmt.Printf("- %s\n", condition)
		}
	}
}

func (m *Menu) listServicesAndIngresses() {
	services, ingresses, err := m.k8sAdapter.GetServicesAndIngresses("default")
	if err != nil {
		fmt.Printf("Ошибка при получении списка сервисов и ингрессов: %v\n", err)
		return
	}

	fmt.Println("\nСервисы:")
	for _, svc := range services {
		fmt.Printf("Имя: %s\n", svc.Name)
		fmt.Printf("Тип: %s\n", svc.Type)
		fmt.Printf("Cluster IP: %s\n", svc.ClusterIP)
		if svc.ExternalIP != "" {
			fmt.Printf("External IP: %s\n", svc.ExternalIP)
		}
		fmt.Printf("Порты: %v\n", svc.Ports)
		fmt.Printf("Возраст: %s\n", svc.Age.Round(time.Second))
		fmt.Println("---")
	}

	fmt.Println("\nИнгрессы:")
	for _, ing := range ingresses {
		fmt.Printf("Имя: %s\n", ing.Name)
		fmt.Printf("Хосты: %v\n", ing.Hosts)
		if len(ing.Addresses) > 0 {
			fmt.Printf("Адреса: %v\n", ing.Addresses)
		}
		fmt.Printf("Возраст: %s\n", ing.Age.Round(time.Second))
		fmt.Println("---")
	}
}

func (m *Menu) deleteResource() {
	fmt.Println("\nДоступные типы ресурсов:")
	fmt.Println("1. Pod")
	fmt.Println("2. Deployment")
	fmt.Println("3. Service")
	fmt.Println("4. ConfigMap")
	fmt.Print("Выберите тип ресурса (1-4): ")

	choice := m.readInput()
	var resourceType string
	var name string

	switch choice {
	case "1":
		resourceType = "pod"
	case "2":
		resourceType = "deployment"
	case "3":
		resourceType = "service"
	case "4":
		resourceType = "configmap"
	default:
		fmt.Println("Неверный выбор")
		return
	}

	// Если выбран ConfigMap, показываем список доступных ConfigMap
	if resourceType == "configmap" {
		configMaps, err := m.k8sAdapter.ListConfigMaps("default")
		if err != nil {
			fmt.Printf("Ошибка при получении списка ConfigMap: %v\n", err)
			return
		}

		if len(configMaps) == 0 {
			fmt.Println("ConfigMap не найдены в namespace default")
			return
		}

		fmt.Println("\nДоступные ConfigMap:")
		for i, cm := range configMaps {
			fmt.Printf("%d. %s (ключи: %v)\n", i+1, cm.Name, cm.Keys)
		}

		fmt.Print("\nВыберите номер ConfigMap для удаления: ")
		numStr := m.readInput()
		num, err := strconv.Atoi(numStr)
		if err != nil || num < 1 || num > len(configMaps) {
			fmt.Println("Неверный номер")
			return
		}

		name = configMaps[num-1].Name
	} else {
		fmt.Print("Введите имя ресурса: ")
		name = m.readInput()
	}

	// Запрашиваем подтверждение
	fmt.Printf("\nВы уверены, что хотите удалить %s '%s'? (y/N): ", resourceType, name)
	confirm := m.readInput()
	if strings.ToLower(confirm) != "y" {
		fmt.Println("Удаление отменено")
		return
	}

	err := m.k8sAdapter.DeleteResource("default", resourceType, name)
	if err != nil {
		fmt.Printf("Ошибка при удалении ресурса: %v\n", err)
		return
	}
	fmt.Printf("%s '%s' успешно удален\n", resourceType, name)
}

func (m *Menu) manageSecret() {
	fmt.Println("\n=== Управление Secret ===")
	fmt.Println("1. Создать/обновить Secret")
	fmt.Println("2. Просмотреть Secret")
	fmt.Println("0. Назад")
	fmt.Print("Выберите действие: ")

	choice := m.readInput()
	switch choice {
	case "1":
		m.createOrUpdateSecret()
	case "2":
		m.viewSecret()
	case "0":
		return
	default:
		fmt.Println("Неверный выбор")
	}
}

func (m *Menu) createOrUpdateSecret() {
	fmt.Print("Введите имя секрета: ")
	name := m.readInput()

	fmt.Println("\nДоступные типы секретов:")
	fmt.Println("1. Opaque (обычный секрет)")
	fmt.Println("2. kubernetes.io/tls (TLS сертификат)")
	fmt.Println("3. kubernetes.io/dockerconfigjson (Docker Registry)")
	fmt.Print("Выберите тип секрета (1-3): ")

	choice := m.readInput()
	var secretType string

	switch choice {
	case "1":
		secretType = "Opaque"
	case "2":
		secretType = "kubernetes.io/tls"
	case "3":
		secretType = "kubernetes.io/dockerconfigjson"
	default:
		fmt.Println("Неверный выбор")
		return
	}

	data := make(map[string][]byte)
	fmt.Println("\nВведите данные (формат: KEY=VALUE, пустая строка для завершения):")
	for {
		line := m.readInput()
		if line == "" {
			break
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			data[parts[0]] = []byte(parts[1])
		}
	}

	err := m.k8sAdapter.CreateOrUpdateSecret("default", name, secretType, data)
	if err != nil {
		fmt.Printf("Ошибка при создании/обновлении секрета: %v\n", err)
		return
	}
	fmt.Println("Секрет успешно создан/обновлен")
}

func (m *Menu) viewSecret() {
	// Сначала показываем список секретов
	secrets, err := m.k8sAdapter.ListSecrets("default")
	if err != nil {
		fmt.Printf("Ошибка при получении списка секретов: %v\n", err)
		return
	}

	if len(secrets) == 0 {
		fmt.Println("Секреты не найдены в namespace default")
		return
	}

	fmt.Println("\nДоступные секреты:")
	for i, secret := range secrets {
		fmt.Printf("%d. %s (тип: %s, ключи: %v)\n", i+1, secret.Name, secret.Type, secret.Keys)
	}

	fmt.Print("\nВыберите номер секрета для просмотра: ")
	numStr := m.readInput()
	num, err := strconv.Atoi(numStr)
	if err != nil || num < 1 || num > len(secrets) {
		fmt.Println("Неверный номер")
		return
	}

	name := secrets[num-1].Name
	info, err := m.k8sAdapter.GetSecretInfo("default", name)
	if err != nil {
		fmt.Printf("Ошибка при получении информации о секрете: %v\n", err)
		return
	}

	fmt.Printf("\nСекрет: %s\n", info.Name)
	fmt.Printf("Namespace: %s\n", info.Namespace)
	fmt.Printf("Тип: %s\n", info.Type)
	fmt.Printf("Возраст: %s\n", info.Age.Round(time.Second))
	fmt.Printf("Ключи: %v\n", info.Keys)
}

func (m *Menu) listSecrets() {
	secrets, err := m.k8sAdapter.ListSecrets("default")
	if err != nil {
		fmt.Printf("Ошибка при получении списка секретов: %v\n", err)
		return
	}

	if len(secrets) == 0 {
		fmt.Println("Секреты не найдены в namespace default")
		return
	}

	fmt.Println("\nСписок секретов:")
	for _, secret := range secrets {
		fmt.Printf("\nИмя: %s\n", secret.Name)
		fmt.Printf("Namespace: %s\n", secret.Namespace)
		fmt.Printf("Тип: %s\n", secret.Type)
		fmt.Printf("Возраст: %s\n", secret.Age.Round(time.Second))
		fmt.Printf("Ключи: %v\n", secret.Keys)
		fmt.Println("---")
	}
}

// CI/CD методы
func (m *Menu) triggerPipeline() {
	if m.cicdAdapter == nil {
		fmt.Println("Ошибка: CI/CD адаптер не инициализирован")
		return
	}

	fmt.Print("Введите ID проекта: ")
	projectID := m.readInput()
	fmt.Print("Введите ветку или тег: ")
	ref := m.readInput()

	pipeline, err := m.cicdAdapter.TriggerPipeline(context.Background(), projectID, ref)
	if err != nil {
		fmt.Printf("Ошибка при запуске сборки: %v\n", err)
		return
	}
	fmt.Printf("Сборка успешно запущена. ID: %s\n", pipeline.ID)
}

func (m *Menu) getPipelineStatus() {
	fmt.Print("Введите ID проекта: ")
	projectID := m.readInput()
	fmt.Print("Введите ID сборки: ")
	pipelineID := m.readInput()

	status, err := m.cicdAdapter.GetPipelineStatus(context.Background(), projectID, pipelineID)
	if err != nil {
		fmt.Printf("Ошибка при получении статуса сборки: %v\n", err)
		return
	}

	fmt.Printf("\nСтатус сборки: %s\n", status.Status)
	fmt.Printf("Начало: %s\n", status.StartedAt.Format(time.RFC3339))
	if !status.EndedAt.IsZero() {
		fmt.Printf("Окончание: %s\n", status.EndedAt.Format(time.RFC3339))
	}
	fmt.Printf("Длительность: %s\n", status.Duration.Round(time.Second))
	fmt.Printf("Автор: %s\n", status.Author)
	fmt.Printf("Сообщение: %s\n", status.Message)
}

func (m *Menu) listPipelineJobs() {
	fmt.Print("Введите ID проекта: ")
	projectID := m.readInput()
	fmt.Print("Введите ID сборки: ")
	pipelineID := m.readInput()

	jobs, err := m.cicdAdapter.ListPipelineJobs(context.Background(), projectID, pipelineID)
	if err != nil {
		fmt.Printf("Ошибка при получении списка задач: %v\n", err)
		return
	}

	fmt.Println("\nСписок задач:")
	for _, job := range jobs {
		fmt.Printf("ID: %s\n", job.ID)
		fmt.Printf("Имя: %s\n", job.Name)
		fmt.Printf("Статус: %s\n", job.Status)
		fmt.Printf("Этап: %s\n", job.Stage)
		fmt.Printf("Начало: %s\n", job.StartedAt.Format(time.RFC3339))
		if !job.EndedAt.IsZero() {
			fmt.Printf("Окончание: %s\n", job.EndedAt.Format(time.RFC3339))
		}
		fmt.Printf("Длительность: %s\n", job.Duration.Round(time.Second))
		fmt.Println("---")
	}
}

func (m *Menu) viewJobLogs() {
	fmt.Print("Введите ID проекта: ")
	projectID := m.readInput()
	fmt.Print("Введите ID задачи: ")
	jobID := m.readInput()

	logs, err := m.cicdAdapter.GetJobLogs(context.Background(), projectID, jobID)
	if err != nil {
		fmt.Printf("Ошибка при получении логов: %v\n", err)
		return
	}

	fmt.Println("\nЛоги задачи:")
	fmt.Println(logs)
}

func (m *Menu) cancelPipeline() {
	fmt.Print("Введите ID проекта: ")
	projectID := m.readInput()
	fmt.Print("Введите ID сборки: ")
	pipelineID := m.readInput()

	err := m.cicdAdapter.CancelPipeline(context.Background(), projectID, pipelineID)
	if err != nil {
		fmt.Printf("Ошибка при отмене сборки: %v\n", err)
		return
	}
	fmt.Println("Сборка успешно отменена")
}

func (m *Menu) retryPipeline() {
	fmt.Print("Введите ID проекта: ")
	projectID := m.readInput()
	fmt.Print("Введите ID сборки: ")
	pipelineID := m.readInput()

	err := m.cicdAdapter.RetryPipeline(context.Background(), projectID, pipelineID)
	if err != nil {
		fmt.Printf("Ошибка при перезапуске сборки: %v\n", err)
		return
	}
	fmt.Println("Сборка успешно перезапущена")
}

func (m *Menu) downloadArtifacts() {
	fmt.Print("Введите ID проекта: ")
	projectID := m.readInput()
	fmt.Print("Введите ID задачи: ")
	jobID := m.readInput()
	fmt.Print("Введите путь для сохранения артефактов: ")
	outputPath := m.readInput()

	err := m.cicdAdapter.DownloadArtifacts(context.Background(), projectID, jobID, outputPath)
	if err != nil {
		fmt.Printf("Ошибка при скачивании артефактов: %v\n", err)
		return
	}
	fmt.Printf("Артефакты успешно скачаны в %s\n", outputPath)
}

// Monitoring методы
func (m *Menu) showRawMetrics() {
	metrics, err := m.monitoringAdapter.GetRawMetrics(context.Background())
	if err != nil {
		fmt.Printf("Ошибка при получении метрик: %v\n", err)
		return
	}

	fmt.Println("\nМетрики:")
	fmt.Println(metrics)
}

func (m *Menu) queryMetric() {
	fmt.Print("Введите имя метрики: ")
	name := m.readInput()

	fmt.Println("\nВыберите временной диапазон:")
	fmt.Println("1. Последние 5 минут")
	fmt.Println("2. Последний час")
	fmt.Println("3. Последние 24 часа")
	fmt.Println("4. Указать свой диапазон")
	fmt.Print("Выберите опцию: ")

	choice := m.readInput()

	var start, end time.Time
	now := time.Now()

	switch choice {
	case "1":
		start = now.Add(-5 * time.Minute)
		end = now
	case "2":
		start = now.Add(-1 * time.Hour)
		end = now
	case "3":
		start = now.Add(-24 * time.Hour)
		end = now
	case "4":
		fmt.Print("Введите начальное время (формат: 2006-01-02 15:04:05): ")
		startStr := m.readInput()
		var err error
		start, err = time.Parse("2006-01-02 15:04:05", startStr)
		if err != nil {
			fmt.Println("Ошибка при разборе начального времени")
			return
		}

		fmt.Print("Введите конечное время (формат: 2006-01-02 15:04:05): ")
		endStr := m.readInput()
		end, err = time.Parse("2006-01-02 15:04:05", endStr)
		if err != nil {
			fmt.Println("Ошибка при разборе конечного времени")
			return
		}
	default:
		fmt.Println("Неверный выбор")
		return
	}

	values, err := m.monitoringAdapter.QueryMetric(context.Background(), name, start, end)
	if err != nil {
		fmt.Printf("Ошибка при запросе метрики: %v\n", err)
		return
	}

	fmt.Printf("\nЗначения метрики %s за период с %s по %s:\n",
		name,
		start.Format("2006-01-02 15:04:05"),
		end.Format("2006-01-02 15:04:05"))

	if len(values) == 0 {
		fmt.Println("Нет данных за указанный период")
		return
	}

	for _, v := range values {
		fmt.Printf("\nВремя: %s\n", v.Timestamp.Format("2006-01-02 15:04:05"))
		fmt.Printf("Значение: %f\n", v.Value)
		if len(v.Labels) > 0 {
			fmt.Println("Метки:")
			for k, v := range v.Labels {
				fmt.Printf("  %s: %s\n", k, v)
			}
		}
		fmt.Println("---")
	}
}

func (m *Menu) listMetrics() {
	fmt.Println("\nДоступные метрики:")
	fmt.Println("\nDocker метрики:")
	fmt.Println("- devops_manager_docker_operations_total - общее количество Docker операций")
	fmt.Println("- devops_manager_docker_operation_duration_seconds - длительность Docker операций")
	fmt.Println("- devops_manager_docker_image_operations_total - операции с образами")
	fmt.Println("- devops_manager_docker_container_operations_total - операции с контейнерами")
	fmt.Println("- devops_manager_docker_network_operations_total - операции с сетями")

	fmt.Println("\nKubernetes метрики:")
	fmt.Println("- devops_manager_kubernetes_operations_total - общее количество Kubernetes операций")
	fmt.Println("- devops_manager_kubernetes_deployment_operations_total - операции с деплойментами")
	fmt.Println("- devops_manager_kubernetes_pod_operations_total - операции с подами")
	fmt.Println("- devops_manager_kubernetes_service_operations_total - операции с сервисами")

	fmt.Println("\nCI/CD метрики:")
	fmt.Println("- devops_manager_cicd_operations_total - общее количество CI/CD операций")
	fmt.Println("- devops_manager_cicd_pipeline_operations_total - операции с пайплайнами")
	fmt.Println("- devops_manager_cicd_job_operations_total - операции с задачами")

	fmt.Println("\nСистемные метрики:")
	fmt.Println("- devops_manager_http_requests_total - количество HTTP запросов")
	fmt.Println("- devops_manager_http_request_duration_seconds - длительность HTTP запросов")
	fmt.Println("- devops_manager_errors_total - количество ошибок")

	fmt.Println("\nДля просмотра значений метрик используйте опцию 'Запрос метрики'")
	fmt.Println("Для просмотра всех метрик используйте опцию 'Сырые метрики'")
}

func (m *Menu) showServiceHealth() {
	health, err := m.monitoringAdapter.GetServiceHealth(context.Background())
	if err != nil {
		fmt.Printf("Ошибка при проверке здоровья сервиса: %v\n", err)
		return
	}

	fmt.Println("\nПроверка здоровья сервиса:")
	for _, check := range health {
		fmt.Printf("Сервис: %s\n", check.Name)
		fmt.Printf("Статус: %s\n", check.Status)
		fmt.Printf("Сообщение: %s\n", check.Message)
		fmt.Printf("Время проверки: %s\n", check.Timestamp.Format(time.RFC3339))
		fmt.Println("---")
	}
}

func (m *Menu) restartContainer() {
	fmt.Print("Введите имя контейнера: ")
	containerName := m.readInput()
	fmt.Print("Введите таймаут в секундах (или оставьте пустым для значения по умолчанию): ")
	timeoutStr := m.readInput()

	containerID, err := m.dockerAdapter.GetContainerIDByName(containerName)
	if err != nil {
		fmt.Printf("Ошибка: %v\n", err)
		return
	}

	var timeout *time.Duration
	if timeoutStr != "" {
		seconds, err := strconv.Atoi(timeoutStr)
		if err != nil {
			fmt.Println("Ошибка: введите корректное число секунд")
			return
		}
		duration := time.Duration(seconds) * time.Second
		timeout = &duration
	}

	err = m.dockerAdapter.RestartContainer(containerID, timeout)
	if err != nil {
		fmt.Printf("Ошибка при перезапуске контейнера: %v\n", err)
		return
	}
	fmt.Println("Контейнер успешно перезапущен")
}

func (m *Menu) configureNginx() {
	fmt.Print("Введите имя ConfigMap (по умолчанию nginx-config): ")
	name := m.readInput()
	if name == "" {
		name = "nginx-config"
	}

	// Получаем текущую конфигурацию
	config, err := m.k8sAdapter.GetNginxConfig("default", name)
	if err != nil {
		fmt.Printf("Ошибка при получении конфигурации: %v\n", err)
		return
	}

	fmt.Println("\nТекущая конфигурация nginx:")
	fmt.Printf("1. Количество рабочих процессов: %s\n", config.WorkerProcesses)
	fmt.Printf("2. Максимальное количество соединений: %s\n", config.WorkerConnections)
	fmt.Printf("3. Таймаут keepalive: %s\n", config.KeepaliveTimeout)
	fmt.Printf("4. Имя сервера: %s\n", config.ServerName)
	fmt.Printf("5. Путь к корневой директории: %s\n", config.RootPath)
	fmt.Printf("6. Файл индекса: %s\n", config.IndexFile)

	fmt.Println("\nВыберите параметр для изменения (1-6) или 0 для выхода:")
	choice := m.readInput()

	switch choice {
	case "1":
		fmt.Print("Введите новое количество рабочих процессов (например, auto или число): ")
		config.WorkerProcesses = m.readInput()
	case "2":
		fmt.Print("Введите новое максимальное количество соединений: ")
		config.WorkerConnections = m.readInput()
	case "3":
		fmt.Print("Введите новый таймаут keepalive (в секундах): ")
		config.KeepaliveTimeout = m.readInput()
	case "4":
		fmt.Print("Введите новое имя сервера: ")
		config.ServerName = m.readInput()
	case "5":
		fmt.Print("Введите новый путь к корневой директории: ")
		config.RootPath = m.readInput()
	case "6":
		fmt.Print("Введите новое имя файла индекса: ")
		config.IndexFile = m.readInput()
	case "0":
		return
	default:
		fmt.Println("Неверный выбор")
		return
	}

	// Обновляем конфигурацию
	err = m.k8sAdapter.UpdateNginxConfig("default", name, config)
	if err != nil {
		fmt.Printf("Ошибка при обновлении конфигурации: %v\n", err)
		return
	}

	fmt.Println("Конфигурация успешно обновлена")
	fmt.Println("Для применения изменений может потребоваться перезапуск подов")
}

func (m *Menu) createOrUpdateConfigMap() {
	fmt.Print("Введите имя ConfigMap: ")
	name := m.readInput()

	data := make(map[string]string)
	fmt.Println("Введите данные (формат: KEY=VALUE, пустая строка для завершения):")
	for {
		line := m.readInput()
		if line == "" {
			break
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			data[parts[0]] = parts[1]
		}
	}

	err := m.k8sAdapter.CreateOrUpdateConfigMap("default", name, data)
	if err != nil {
		fmt.Printf("Ошибка при создании/обновлении ConfigMap: %v\n", err)
		return
	}
	fmt.Println("ConfigMap успешно создан/обновлен")
}

func (m *Menu) viewConfigMap() {
	fmt.Print("Введите имя ConfigMap: ")
	name := m.readInput()

	info, err := m.k8sAdapter.GetConfigMapInfo("default", name)
	if err != nil {
		fmt.Printf("Ошибка при получении информации о ConfigMap: %v\n", err)
		return
	}

	fmt.Printf("\nConfigMap: %s\n", info.Name)
	fmt.Printf("Namespace: %s\n", info.Namespace)
	fmt.Printf("Возраст: %s\n", info.Age.Round(time.Second))
	fmt.Println("\nДанные:")
	for key, value := range info.Data {
		fmt.Printf("%s: %s\n", key, value)
	}
}

func (m *Menu) listConfigMaps() {
	configMaps, err := m.k8sAdapter.ListConfigMaps("default")
	if err != nil {
		fmt.Printf("Ошибка при получении списка ConfigMap: %v\n", err)
		return
	}

	if len(configMaps) == 0 {
		fmt.Println("ConfigMap не найдены в namespace default")
		return
	}

	fmt.Println("\nСписок ConfigMap:")
	for _, cm := range configMaps {
		fmt.Printf("\nИмя: %s\n", cm.Name)
		fmt.Printf("Namespace: %s\n", cm.Namespace)
		fmt.Printf("Возраст: %s\n", cm.Age.Round(time.Second))
		fmt.Printf("Ключи: %v\n", cm.Keys)
		fmt.Println("---")
	}
}

func (m *Menu) configureGitLabCI() {
	fmt.Println("\n=== Настройка .gitlab-ci.yml ===")
	fmt.Println("1. Создать/обновить .gitlab-ci.yml")
	fmt.Println("2. Просмотреть текущий .gitlab-ci.yml")
	fmt.Println("0. Назад")
	fmt.Print("Выберите действие: ")

	choice := m.readInput()
	switch choice {
	case "1":
		m.createOrUpdateGitLabCI()
	case "2":
		m.viewGitLabCI()
	case "0":
		return
	default:
		fmt.Println("Неверный выбор")
	}
}

func (m *Menu) createOrUpdateGitLabCI() {
	fmt.Print("Введите имя .gitlab-ci.yml: ")
	name := m.readInput()

	data := make(map[string]string)
	fmt.Println("Введите данные (формат: KEY=VALUE, пустая строка для завершения):")
	for {
		line := m.readInput()
		if line == "" {
			break
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			data[parts[0]] = parts[1]
		}
	}

	err := m.cicdAdapter.CreateOrUpdateGitLabCI(name, data)
	if err != nil {
		fmt.Printf("Ошибка при создании/обновлении .gitlab-ci.yml: %v\n", err)
		return
	}
	fmt.Println("Файл .gitlab-ci.yml успешно создан/обновлен")
}

func (m *Menu) viewGitLabCI() {
	content, err := m.cicdAdapter.GetGitLabCI()
	if err != nil {
		fmt.Printf("Ошибка при получении содержимого .gitlab-ci.yml: %v\n", err)
		return
	}

	fmt.Println("\nСодержимое .gitlab-ci.yml:")
	fmt.Println(content)
}

func main() {
	menu, err := NewMenu()
	if err != nil {
		fmt.Printf("Ошибка при инициализации меню: %v\n", err)
		os.Exit(1)
	}
	defer menu.dockerAdapter.Close()

	for {
		menu.printMainMenu()
		choice := menu.readInput()

		switch choice {
		case "1":
			menu.handleImageMenu()
		case "2":
			menu.handleContainerMenu()
		case "3":
			menu.handleKubernetesMenu()
		case "4":
			menu.handleCICDMenu()
		case "5":
			menu.handleMonitoringMenu()
		case "0":
			fmt.Println("Выход из программы")
			return
		default:
			fmt.Println("Неверный выбор")
		}
	}
}
