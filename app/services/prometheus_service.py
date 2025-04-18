import aiohttp
from typing import Dict, List, Optional
from datetime import datetime, timedelta

class PrometheusService:
    def __init__(self, prometheus_url: str):
        self.prometheus_url = prometheus_url.rstrip('/')
        
    async def query(self, query: str, time_range: Optional[str] = None) -> Dict:
        """Выполнение запроса к Prometheus"""
        params = {'query': query}
        if time_range:
            params['time'] = time_range
            
        async with aiohttp.ClientSession() as session:
            async with session.get(f"{self.prometheus_url}/api/v1/query", params=params) as response:
                if response.status != 200:
                    raise Exception(f"Ошибка запроса к Prometheus: {await response.text()}")
                return await response.json()
    
    async def get_cluster_metrics(self, cluster_name: str) -> Dict:
        """Получение метрик кластера"""
        now = datetime.now()
        hour_ago = now - timedelta(hours=1)
        
        metrics = {
            "cpu_usage": await self.query(
                f'sum(rate(container_cpu_usage_seconds_total{{cluster="{cluster_name}"}}[5m]))'
            ),
            "memory_usage": await self.query(
                f'sum(container_memory_usage_bytes{{cluster="{cluster_name}"}})'
            ),
            "pod_count": await self.query(
                f'count(kube_pod_info{{cluster="{cluster_name}"}})'
            ),
            "node_count": await self.query(
                f'count(kube_node_info{{cluster="{cluster_name}"}})'
            )
        }
        
        return {
            "timestamp": now.isoformat(),
            "metrics": metrics
        }
    
    async def get_pod_metrics(self, cluster_name: str, namespace: str, pod_name: str) -> Dict:
        """Получение метрик пода"""
        metrics = {
            "cpu": await self.query(
                f'rate(container_cpu_usage_seconds_total{{cluster="{cluster_name}", namespace="{namespace}", pod="{pod_name}"}}[5m])'
            ),
            "memory": await self.query(
                f'container_memory_usage_bytes{{cluster="{cluster_name}", namespace="{namespace}", pod="{pod_name}"}}'
            ),
            "network": await self.query(
                f'rate(container_network_receive_bytes_total{{cluster="{cluster_name}", namespace="{namespace}", pod="{pod_name}"}}[5m])'
            )
        }
        
        return {
            "timestamp": datetime.now().isoformat(),
            "metrics": metrics
        } 