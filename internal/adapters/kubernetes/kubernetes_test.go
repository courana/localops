package kubernetes

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	clusterName = "test-cluster"
)

func setupKindCluster(t *testing.T) (string, func()) {
	// Создаем конфигурацию для kind
	config := `kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  image: kindest/node:v1.29.0`

	configPath := filepath.Join(t.TempDir(), "kind-config.yaml")
	err := os.WriteFile(configPath, []byte(config), 0644)
	require.NoError(t, err)

	// Создаем кластер
	cmd := exec.Command("kind", "create", "cluster", "--name", clusterName, "--config", configPath)
	err = cmd.Run()
	require.NoError(t, err)

	// Получаем kubeconfig
	cmd = exec.Command("kind", "get", "kubeconfig", "--name", clusterName)
	kubeconfig, err := cmd.Output()
	require.NoError(t, err)

	kubeconfigPath := filepath.Join(t.TempDir(), "kubeconfig")
	err = os.WriteFile(kubeconfigPath, kubeconfig, 0644)
	require.NoError(t, err)

	// Функция очистки
	cleanup := func() {
		exec.Command("kind", "delete", "cluster", "--name", clusterName).Run()
	}

	return kubeconfigPath, cleanup
}

func TestK8sAdapter_DeployScaleDelete(t *testing.T) {
	kubeconfigPath, cleanup := setupKindCluster(t)
	defer cleanup()

	adapter, err := NewK8sAdapter(kubeconfigPath)
	require.NoError(t, err)

	ctx := context.Background()

	// Тест ApplyManifest
	t.Run("ApplyManifest", func(t *testing.T) {
		manifestPath := filepath.Join("testdata", "deployment.yaml")
		err := adapter.ApplyManifest(manifestPath)
		assert.NoError(t, err)

		// Ждем, пока Deployment будет готов
		time.Sleep(5 * time.Second)

		// Проверяем, что Deployment создан
		config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		require.NoError(t, err)

		clientset, err := kubernetes.NewForConfig(config)
		require.NoError(t, err)

		deployment, err := clientset.AppsV1().Deployments("default").Get(ctx, "test-deployment", metav1.GetOptions{})
		assert.NoError(t, err)
		assert.Equal(t, int32(1), *deployment.Spec.Replicas)
	})

	// Тест Scale
	t.Run("Scale", func(t *testing.T) {
		err := adapter.Scale("default", "test-deployment", 3)
		assert.NoError(t, err)

		// Ждем, пока масштабирование завершится
		time.Sleep(5 * time.Second)

		// Проверяем, что количество реплик изменилось
		config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		require.NoError(t, err)

		clientset, err := kubernetes.NewForConfig(config)
		require.NoError(t, err)

		deployment, err := clientset.AppsV1().Deployments("default").Get(ctx, "test-deployment", metav1.GetOptions{})
		assert.NoError(t, err)
		assert.Equal(t, int32(3), *deployment.Spec.Replicas)
	})

	// Тест DeleteResource
	t.Run("DeleteResource", func(t *testing.T) {
		err := adapter.DeleteResource("default", "deployment", "test-deployment")
		assert.NoError(t, err)

		// Ждем, пока удаление завершится
		time.Sleep(5 * time.Second)

		// Проверяем, что Deployment удален
		config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		require.NoError(t, err)

		clientset, err := kubernetes.NewForConfig(config)
		require.NoError(t, err)

		_, err = clientset.AppsV1().Deployments("default").Get(ctx, "test-deployment", metav1.GetOptions{})
		assert.Error(t, err)
	})
}
