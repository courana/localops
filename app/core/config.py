from pydantic_settings import BaseSettings
from typing import List, Optional

class Settings(BaseSettings):
    # Основные настройки
    APP_NAME: str = "TheTool"
    DEBUG: bool = False
    
    # Настройки Kubernetes
    KUBE_CONFIG_PATH: str = "~/.kube/config"
    
    # Настройки Prometheus
    PROMETHEUS_URL: str = "http://localhost:9090"
    
    # Настройки Jenkins
    JENKINS_URL: Optional[str] = None
    JENKINS_USERNAME: Optional[str] = None
    JENKINS_TOKEN: Optional[str] = None
    
    # Настройки GitLab
    GITLAB_URL: Optional[str] = None
    GITLAB_TOKEN: Optional[str] = None
    
    # Список кластеров
    CLUSTERS: List[dict] = []
    
    class Config:
        env_file = ".env"

settings = Settings() 