from kubernetes import client, config
from kubernetes.client.rest import ApiException
from typing import List, Dict, Optional
import yaml

class KubernetesService:
    def __init__(self, kubeconfig: str):
        self.kubeconfig = kubeconfig
        self.api_client = self._get_api_client()
        
    def _get_api_client(self):
        """Инициализация клиента Kubernetes"""
        try:
            config.load_kube_config(config_file=self.kubeconfig)
            return client.ApiClient()
        except Exception as e:
            raise Exception(f"Ошибка инициализации Kubernetes клиента: {str(e)}")
    
    def get_cluster_info(self) -> Dict:
        """Получение информации о кластере"""
        v1 = client.CoreV1Api(self.api_client)
        try:
            nodes = v1.list_node()
            namespaces = v1.list_namespace()
            return {
                "node_count": len(nodes.items),
                "namespaces": [ns.metadata.name for ns in namespaces.items]
            }
        except ApiException as e:
            raise Exception(f"Ошибка получения информации о кластере: {str(e)}")
    
    def get_pods(self, namespace: str = "default") -> List[Dict]:
        """Получение списка подов"""
        v1 = client.CoreV1Api(self.api_client)
        try:
            pods = v1.list_namespaced_pod(namespace)
            return [{
                "name": pod.metadata.name,
                "status": pod.status.phase,
                "containers": [c.name for c in pod.spec.containers]
            } for pod in pods.items]
        except ApiException as e:
            raise Exception(f"Ошибка получения списка подов: {str(e)}")
    
    def apply_manifest(self, manifest: str) -> Dict:
        """Применение манифеста"""
        try:
            manifest_dict = yaml.safe_load(manifest)
            api_version = manifest_dict.get("apiVersion", "").split("/")[0]
            kind = manifest_dict.get("kind", "").lower()
            
            if kind == "deployment":
                api = client.AppsV1Api(self.api_client)
                return api.create_namespaced_deployment(
                    body=manifest_dict,
                    namespace=manifest_dict.get("metadata", {}).get("namespace", "default")
                )
            # Добавить обработку других типов ресурсов
            
        except Exception as e:
            raise Exception(f"Ошибка применения манифеста: {str(e)}") 