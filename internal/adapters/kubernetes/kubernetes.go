package kubernetes

import (
	"context"
	"fmt"
	"io/ioutil"
	"time"

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

	// Декодируем YAML в Unstructured
	obj := &unstructured.Unstructured{}
	if err := yaml.Unmarshal(data, obj); err != nil {
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
			return fmt.Errorf("ошибка при создании ресурса: %w", err)
		}
	} else {
		// Если ресурс существует, обновляем его
		_, err = dynamicResource.Namespace(obj.GetNamespace()).Update(k.ctx, obj, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("ошибка при обновлении ресурса: %w", err)
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
	default:
		return fmt.Errorf("неподдерживаемый тип ресурса: %s", resourceType)
	}
}
