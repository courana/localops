package kubernetes

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
)

// PodStatus содержит информацию о состоянии пода
type PodStatus struct {
	Name      string
	Namespace string
	Status    string
	Ready     bool
	Age       time.Duration
	IP        string
	Node      string
	Restarts  int32
}

// DeploymentStatus содержит информацию о состоянии деплоймента
type DeploymentStatus struct {
	Name                string
	Namespace           string
	Replicas            int32
	ReadyReplicas       int32
	UpdatedReplicas     int32
	AvailableReplicas   int32
	UnavailableReplicas int32
	Conditions          []string
}

// ServiceInfo содержит информацию о сервисе
type ServiceInfo struct {
	Name       string
	Namespace  string
	Type       string
	ClusterIP  string
	ExternalIP string
	Ports      []string
	Age        time.Duration
}

// IngressInfo содержит информацию об ингрессе
type IngressInfo struct {
	Name      string
	Namespace string
	Hosts     []string
	Addresses []string
	Age       time.Duration
}

// ConfigMapInfo содержит информацию о ConfigMap
type ConfigMapInfo struct {
	Name      string
	Namespace string
	Data      map[string]string
	Age       time.Duration
}

// SecretInfo содержит информацию о Secret
type SecretInfo struct {
	Name      string
	Namespace string
	Type      string
	Keys      []string
	Age       time.Duration
}

// NginxConfig содержит настройки nginx
type NginxConfig struct {
	WorkerProcesses   string
	WorkerConnections string
	KeepaliveTimeout  string
	ServerName        string
	RootPath          string
	IndexFile         string
}

// ConfigMapListItem содержит базовую информацию о ConfigMap
type ConfigMapListItem struct {
	Name      string
	Namespace string
	Age       time.Duration
	Keys      []string
}

// SecretListItem содержит базовую информацию о Secret
type SecretListItem struct {
	Name      string
	Namespace string
	Type      string
	Age       time.Duration
	Keys      []string
}

// K8sAdapter предоставляет методы для работы с Kubernetes
type K8sAdapter struct {
	clientset *kubernetes.Clientset
	dynamic   dynamic.Interface
	ctx       context.Context
}

// NewK8sAdapter создает новый экземпляр K8sAdapter
func NewK8sAdapter(kubeconfigPath string) (*K8sAdapter, error) {
	// Загружаем конфигурацию из файла
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("ошибка при загрузке конфигурации: %w", err)
	}

	// Создаем typed клиент
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("ошибка при создании typed клиента: %w", err)
	}

	// Создаем dynamic клиент
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("ошибка при создании dynamic клиента: %w", err)
	}

	return &K8sAdapter{
		clientset: clientset,
		dynamic:   dynamicClient,
		ctx:       context.Background(),
	}, nil
}

// ApplyManifest применяет YAML манифест к кластеру
func (k *K8sAdapter) ApplyManifest(manifestPath string) error {
	// Читаем YAML файл
	data, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("ошибка при чтении манифеста: %w", err)
	}

	// Разделяем манифест на отдельные ресурсы
	resources := bytes.Split(data, []byte("---"))

	for _, resourceData := range resources {
		if len(bytes.TrimSpace(resourceData)) == 0 {
			continue
		}

		// Декодируем YAML в Unstructured
		obj := &unstructured.Unstructured{}
		if err := yaml.Unmarshal(resourceData, obj); err != nil {
			return fmt.Errorf("ошибка при разборе YAML: %w", err)
		}

		// Получаем GVR (GroupVersionResource) для объекта
		gvk := obj.GetObjectKind().GroupVersionKind()

		// Создаем RESTMapper
		discoveryClient := k.clientset.Discovery()
		groupResources, err := restmapper.GetAPIGroupResources(discoveryClient)
		if err != nil {
			return fmt.Errorf("ошибка при получении API групп: %w", err)
		}
		mapper := restmapper.NewDiscoveryRESTMapper(groupResources)

		// Получаем mapping для ресурса
		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return fmt.Errorf("ошибка при получении mapping: %w", err)
		}

		// Получаем dynamic client для конкретного ресурса
		dynamicResource := k.dynamic.Resource(mapping.Resource)

		// Проверяем существование ресурса
		_, err = dynamicResource.Namespace(obj.GetNamespace()).Get(k.ctx, obj.GetName(), metav1.GetOptions{})
		if err != nil {
			// Если ресурс не существует, создаем его
			_, err = dynamicResource.Namespace(obj.GetNamespace()).Create(k.ctx, obj, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("ошибка при создании ресурса %s: %w", obj.GetName(), err)
			}
			fmt.Printf("Создан ресурс: %s/%s\n", obj.GetKind(), obj.GetName())
		} else {
			// Если ресурс существует, обновляем его
			_, err = dynamicResource.Namespace(obj.GetNamespace()).Update(k.ctx, obj, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("ошибка при обновлении ресурса %s: %w", obj.GetName(), err)
			}
			fmt.Printf("Обновлен ресурс: %s/%s\n", obj.GetKind(), obj.GetName())
		}
	}

	return nil
}

// Scale изменяет количество реплик для деплоймента
func (k *K8sAdapter) Scale(namespace, name string, replicas int32) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		deployment, err := k.clientset.AppsV1().Deployments(namespace).Get(k.ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		deployment.Spec.Replicas = &replicas
		_, err = k.clientset.AppsV1().Deployments(namespace).Update(k.ctx, deployment, metav1.UpdateOptions{})
		return err
	})
}

// GetPodStatus возвращает статус конкретного пода
func (k *K8sAdapter) GetPodStatus(namespace, name string) (*PodStatus, error) {
	pod, err := k.clientset.CoreV1().Pods(namespace).Get(k.ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении пода: %w", err)
	}

	status := &PodStatus{
		Name:      pod.Name,
		Namespace: pod.Namespace,
		Status:    string(pod.Status.Phase),
		IP:        pod.Status.PodIP,
		Node:      pod.Spec.NodeName,
		Age:       time.Since(pod.CreationTimestamp.Time),
	}

	// Проверяем готовность пода
	status.Ready = true
	for _, container := range pod.Status.ContainerStatuses {
		if !container.Ready {
			status.Ready = false
			break
		}
		status.Restarts += container.RestartCount
	}

	return status, nil
}

// GetPodStatuses возвращает статусы всех подов в указанном namespace
func (k *K8sAdapter) GetPodStatuses(namespace string) ([]PodStatus, error) {
	pods, err := k.clientset.CoreV1().Pods(namespace).List(k.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении списка подов: %w", err)
	}

	var statuses []PodStatus
	for _, pod := range pods.Items {
		status := PodStatus{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Status:    string(pod.Status.Phase),
			IP:        pod.Status.PodIP,
			Node:      pod.Spec.NodeName,
			Age:       time.Since(pod.CreationTimestamp.Time),
		}

		// Проверяем готовность пода
		status.Ready = true
		for _, container := range pod.Status.ContainerStatuses {
			if !container.Ready {
				status.Ready = false
				break
			}
			status.Restarts += container.RestartCount
		}

		statuses = append(statuses, status)
	}

	return statuses, nil
}

// DeleteResource удаляет ресурс указанного типа и имени
func (k *K8sAdapter) DeleteResource(namespace, resourceType, name string) error {
	switch resourceType {
	case "deployment":
		return k.clientset.AppsV1().Deployments(namespace).Delete(k.ctx, name, metav1.DeleteOptions{})
	case "service":
		return k.clientset.CoreV1().Services(namespace).Delete(k.ctx, name, metav1.DeleteOptions{})
	case "pod":
		return k.clientset.CoreV1().Pods(namespace).Delete(k.ctx, name, metav1.DeleteOptions{})
	case "configmap":
		return k.clientset.CoreV1().ConfigMaps(namespace).Delete(k.ctx, name, metav1.DeleteOptions{})
	default:
		return fmt.Errorf("неподдерживаемый тип ресурса: %s", resourceType)
	}
}

// GetDeploymentStatus возвращает статус деплоймента
func (k *K8sAdapter) GetDeploymentStatus(namespace, name string) (*DeploymentStatus, error) {
	deployment, err := k.clientset.AppsV1().Deployments(namespace).Get(k.ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении деплоймента: %w", err)
	}

	status := &DeploymentStatus{
		Name:                deployment.Name,
		Namespace:           deployment.Namespace,
		Replicas:            *deployment.Spec.Replicas,
		ReadyReplicas:       deployment.Status.ReadyReplicas,
		UpdatedReplicas:     deployment.Status.UpdatedReplicas,
		AvailableReplicas:   deployment.Status.AvailableReplicas,
		UnavailableReplicas: deployment.Status.UnavailableReplicas,
	}

	// Добавляем условия деплоймента
	for _, condition := range deployment.Status.Conditions {
		status.Conditions = append(status.Conditions, fmt.Sprintf("%s: %s", condition.Type, condition.Status))
	}

	return status, nil
}

// GetServicesAndIngresses возвращает информацию о сервисах и ингрессах
func (k *K8sAdapter) GetServicesAndIngresses(namespace string) ([]ServiceInfo, []IngressInfo, error) {
	// Получаем список сервисов
	services, err := k.clientset.CoreV1().Services(namespace).List(k.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("ошибка при получении списка сервисов: %w", err)
	}

	var serviceInfos []ServiceInfo
	for _, svc := range services.Items {
		info := ServiceInfo{
			Name:      svc.Name,
			Namespace: svc.Namespace,
			Type:      string(svc.Spec.Type),
			ClusterIP: svc.Spec.ClusterIP,
			Age:       time.Since(svc.CreationTimestamp.Time),
		}

		// Добавляем внешние IP
		if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
			for _, ingress := range svc.Status.LoadBalancer.Ingress {
				if ingress.IP != "" {
					info.ExternalIP = ingress.IP
				}
			}
		}

		// Добавляем порты
		for _, port := range svc.Spec.Ports {
			portStr := fmt.Sprintf("%d/%s", port.Port, port.Protocol)
			if port.NodePort > 0 {
				portStr = fmt.Sprintf("%s:%d", portStr, port.NodePort)
			}
			info.Ports = append(info.Ports, portStr)
		}

		serviceInfos = append(serviceInfos, info)
	}

	// Получаем список ингрессов
	ingresses, err := k.clientset.NetworkingV1().Ingresses(namespace).List(k.ctx, metav1.ListOptions{})
	if err != nil {
		// Если ошибка связана с тем, что API не поддерживается, возвращаем только сервисы
		if errors.IsNotFound(err) {
			return serviceInfos, nil, nil
		}
		return serviceInfos, nil, fmt.Errorf("ошибка при получении списка ингрессов: %w", err)
	}

	var ingressInfos []IngressInfo
	for _, ing := range ingresses.Items {
		info := IngressInfo{
			Name:      ing.Name,
			Namespace: ing.Namespace,
			Age:       time.Since(ing.CreationTimestamp.Time),
		}

		// Добавляем хосты
		for _, rule := range ing.Spec.Rules {
			if rule.Host != "" {
				info.Hosts = append(info.Hosts, rule.Host)
			}
		}

		// Добавляем адреса
		for _, lb := range ing.Status.LoadBalancer.Ingress {
			if lb.IP != "" {
				info.Addresses = append(info.Addresses, lb.IP)
			}
			if lb.Hostname != "" {
				info.Addresses = append(info.Addresses, lb.Hostname)
			}
		}

		ingressInfos = append(ingressInfos, info)
	}

	return serviceInfos, ingressInfos, nil
}

// CreateOrUpdateConfigMap создает или обновляет ConfigMap
func (k *K8sAdapter) CreateOrUpdateConfigMap(namespace, name string, data map[string]string) error {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}

	_, err := k.clientset.CoreV1().ConfigMaps(namespace).Get(k.ctx, name, metav1.GetOptions{})
	if err != nil {
		// Если ConfigMap не существует, создаем его
		_, err = k.clientset.CoreV1().ConfigMaps(namespace).Create(k.ctx, configMap, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("ошибка при создании ConfigMap: %w", err)
		}
	} else {
		// Если ConfigMap существует, обновляем его
		_, err = k.clientset.CoreV1().ConfigMaps(namespace).Update(k.ctx, configMap, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("ошибка при обновлении ConfigMap: %w", err)
		}
	}

	return nil
}

// CreateOrUpdateSecret создает или обновляет Secret
func (k *K8sAdapter) CreateOrUpdateSecret(namespace, name, secretType string, data map[string][]byte) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Type: corev1.SecretType(secretType),
		Data: data,
	}

	_, err := k.clientset.CoreV1().Secrets(namespace).Get(k.ctx, name, metav1.GetOptions{})
	if err != nil {
		// Если Secret не существует, создаем его
		_, err = k.clientset.CoreV1().Secrets(namespace).Create(k.ctx, secret, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("ошибка при создании Secret: %w", err)
		}
	} else {
		// Если Secret существует, обновляем его
		_, err = k.clientset.CoreV1().Secrets(namespace).Update(k.ctx, secret, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("ошибка при обновлении Secret: %w", err)
		}
	}

	return nil
}

// GetConfigMapInfo возвращает информацию о ConfigMap
func (k *K8sAdapter) GetConfigMapInfo(namespace, name string) (*ConfigMapInfo, error) {
	configMap, err := k.clientset.CoreV1().ConfigMaps(namespace).Get(k.ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении ConfigMap: %w", err)
	}

	return &ConfigMapInfo{
		Name:      configMap.Name,
		Namespace: configMap.Namespace,
		Data:      configMap.Data,
		Age:       time.Since(configMap.CreationTimestamp.Time),
	}, nil
}

// GetSecretInfo возвращает информацию о Secret
func (k *K8sAdapter) GetSecretInfo(namespace, name string) (*SecretInfo, error) {
	secret, err := k.clientset.CoreV1().Secrets(namespace).Get(k.ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении Secret: %w", err)
	}

	keys := make([]string, 0, len(secret.Data))
	for key := range secret.Data {
		keys = append(keys, key)
	}

	return &SecretInfo{
		Name:      secret.Name,
		Namespace: secret.Namespace,
		Type:      string(secret.Type),
		Keys:      keys,
		Age:       time.Since(secret.CreationTimestamp.Time),
	}, nil
}

// GetNginxConfig возвращает текущую конфигурацию nginx
func (k *K8sAdapter) GetNginxConfig(namespace, configMapName string) (*NginxConfig, error) {
	configMap, err := k.clientset.CoreV1().ConfigMaps(namespace).Get(k.ctx, configMapName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении ConfigMap: %w", err)
	}

	nginxConf := configMap.Data["nginx.conf"]
	if nginxConf == "" {
		return nil, fmt.Errorf("nginx.conf не найден в ConfigMap")
	}

	// Парсим конфигурацию
	config := &NginxConfig{
		WorkerProcesses:   "auto",
		WorkerConnections: "1024",
		KeepaliveTimeout:  "65",
		ServerName:        "localhost",
		RootPath:          "/usr/share/nginx/html",
		IndexFile:         "index.html",
	}

	// TODO: Добавить парсинг конфигурации из nginxConf

	return config, nil
}

// UpdateNginxConfig обновляет конфигурацию nginx
func (k *K8sAdapter) UpdateNginxConfig(namespace, configMapName string, config *NginxConfig) error {
	// Получаем текущий ConfigMap
	configMap, err := k.clientset.CoreV1().ConfigMaps(namespace).Get(k.ctx, configMapName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("ошибка при получении ConfigMap: %w", err)
	}

	// Формируем новую конфигурацию nginx
	nginxConf := fmt.Sprintf(`user nginx;
worker_processes %s;
error_log /var/log/nginx/error.log;
pid /var/run/nginx.pid;

events {
	worker_connections %s;
}

http {
	include /etc/nginx/mime.types;
	default_type application/octet-stream;
	sendfile on;
	keepalive_timeout %s;

	server {
		listen 80;
		server_name %s;

		location / {
			root %s;
			index %s;
		}
	}
}`, config.WorkerProcesses, config.WorkerConnections, config.KeepaliveTimeout,
		config.ServerName, config.RootPath, config.IndexFile)

	// Обновляем ConfigMap
	configMap.Data["nginx.conf"] = nginxConf

	// Сохраняем изменения
	_, err = k.clientset.CoreV1().ConfigMaps(namespace).Update(k.ctx, configMap, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("ошибка при обновлении ConfigMap: %w", err)
	}

	return nil
}

// ListConfigMaps возвращает список всех ConfigMap в указанном namespace
func (k *K8sAdapter) ListConfigMaps(namespace string) ([]ConfigMapListItem, error) {
	configMaps, err := k.clientset.CoreV1().ConfigMaps(namespace).List(k.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении списка ConfigMap: %w", err)
	}

	var items []ConfigMapListItem
	for _, cm := range configMaps.Items {
		keys := make([]string, 0, len(cm.Data))
		for key := range cm.Data {
			keys = append(keys, key)
		}

		items = append(items, ConfigMapListItem{
			Name:      cm.Name,
			Namespace: cm.Namespace,
			Age:       time.Since(cm.CreationTimestamp.Time),
			Keys:      keys,
		})
	}

	return items, nil
}

// ListSecrets возвращает список всех секретов в указанном namespace
func (k *K8sAdapter) ListSecrets(namespace string) ([]SecretListItem, error) {
	secrets, err := k.clientset.CoreV1().Secrets(namespace).List(k.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении списка секретов: %w", err)
	}

	var items []SecretListItem
	for _, secret := range secrets.Items {
		keys := make([]string, 0, len(secret.Data))
		for key := range secret.Data {
			keys = append(keys, key)
		}

		items = append(items, SecretListItem{
			Name:      secret.Name,
			Namespace: secret.Namespace,
			Type:      string(secret.Type),
			Age:       time.Since(secret.CreationTimestamp.Time),
			Keys:      keys,
		})
	}

	return items, nil
}
