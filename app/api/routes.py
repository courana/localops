from fastapi import APIRouter, HTTPException, Depends
from typing import List
from app.models.cluster import Cluster, ClusterCreate, ClusterMetrics
from app.services.kubernetes_service import KubernetesService
from app.services.prometheus_service import PrometheusService
from app.core.config import settings

router = APIRouter()

@router.get("/clusters", response_model=List[Cluster])
async def get_clusters():
    """Получение списка всех кластеров"""
    return settings.CLUSTERS

@router.post("/clusters", response_model=Cluster)
async def create_cluster(cluster: ClusterCreate):
    """Создание нового кластера"""
    try:
        k8s_service = KubernetesService(cluster.kubeconfig)
        cluster_info = k8s_service.get_cluster_info()
        
        new_cluster = Cluster(
            id=str(len(settings.CLUSTERS) + 1),
            name=cluster.name,
            description=cluster.description,
            kubeconfig=cluster.kubeconfig,
            prometheus_url=cluster.prometheus_url,
            status="active",
            nodes=cluster_info["nodes"],
            namespaces=cluster_info["namespaces"]
        )
        
        settings.CLUSTERS.append(new_cluster)
        return new_cluster
    except Exception as e:
        raise HTTPException(status_code=400, detail=str(e))

@router.get("/clusters/{cluster_id}/metrics", response_model=ClusterMetrics)
async def get_cluster_metrics(cluster_id: str):
    """Получение метрик кластера"""
    cluster = next((c for c in settings.CLUSTERS if c.id == cluster_id), None)
    if not cluster:
        raise HTTPException(status_code=404, detail="Кластер не найден")
    
    if not cluster.prometheus_url:
        raise HTTPException(status_code=400, detail="Prometheus URL не настроен для этого кластера")
    
    try:
        prometheus_service = PrometheusService(cluster.prometheus_url)
        metrics = await prometheus_service.get_cluster_metrics(cluster.name)
        return ClusterMetrics(**metrics)
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@router.get("/clusters/{cluster_id}/pods")
async def get_cluster_pods(cluster_id: str, namespace: str = "default"):
    """Получение списка подов в кластере"""
    cluster = next((c for c in settings.CLUSTERS if c.id == cluster_id), None)
    if not cluster:
        raise HTTPException(status_code=404, detail="Кластер не найден")
    
    try:
        k8s_service = KubernetesService(cluster.kubeconfig)
        pods = k8s_service.get_pods(namespace)
        return pods
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e)) 