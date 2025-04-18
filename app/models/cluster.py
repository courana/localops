from pydantic import BaseModel
from typing import Optional, List, Dict

class ClusterBase(BaseModel):
    name: str
    description: Optional[str] = None
    kubeconfig: str
    prometheus_url: Optional[str] = None

class ClusterCreate(ClusterBase):
    pass

class Cluster(ClusterBase):
    id: str
    status: str
    nodes: List[Dict]
    namespaces: List[str]
    
    class Config:
        from_attributes = True

class ClusterMetrics(BaseModel):
    cpu_usage: float
    memory_usage: float
    pod_count: int
    node_count: int
    timestamp: str 