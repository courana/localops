package api

import (
	"log"
	"net/http"
	"time"

	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	restful "github.com/emicklei/go-restful/v3"
	"github.com/go-openapi/spec"
)

// LoggingMiddleware логирует информацию о запросе
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.RequestURI, time.Since(start))
	})
}

// RecoverMiddleware обрабатывает паники
func RecoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// ContainerOptions содержит параметры для запуска контейнера
type ContainerOptions struct {
	Image   string            `json:"image"`
	Name    string            `json:"name,omitempty"`
	Ports   []PortMapping     `json:"ports,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Volumes map[string]string `json:"volumes,omitempty"`
	Network string            `json:"network,omitempty"`
}

// PortMapping описывает маппинг портов
type PortMapping struct {
	HostPort      int `json:"hostPort"`
	ContainerPort int `json:"containerPort"`
}

func NewAPI(dockerAdapter, k8sAdapter, ciAdapter, monitoringAdapter interface{}) http.Handler {
	wsContainer := restful.NewContainer()

	// Docker endpoints
	dockerWS := new(restful.WebService)
	dockerWS.
		Path("/api/docker").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)

	// Docker ping
	dockerWS.Route(dockerWS.GET("/ping").To(dockerPingHandler).Doc("Ping Docker").Operation("dockerPing"))

	// Docker containers
	dockerWS.Route(dockerWS.GET("/containers").To(dockerListContainersHandler).Doc("List Docker Containers").Operation("dockerListContainers"))
	dockerWS.Route(dockerWS.POST("/containers").To(dockerRunContainerHandler).Doc("Run Docker Container").Operation("dockerRunContainer"))

	// Docker images
	dockerWS.Route(dockerWS.POST("/pull").To(dockerPullImageHandler).Doc("Pull Docker Image").Operation("dockerPullImage"))

	wsContainer.Add(dockerWS)

	// K8s endpoints
	k8sWS := new(restful.WebService)
	k8sWS.
		Path("/api/k8s").
		Consumes(restful.MIME_JSON, "application/yaml").
		Produces(restful.MIME_JSON)

	k8sWS.Route(k8sWS.GET("/ping").To(k8sPingHandler).Doc("Ping K8s").Operation("k8sPing"))
	k8sWS.Route(k8sWS.POST("/deploy").To(k8sDeployHandler).Doc("Deploy to Kubernetes").Operation("k8sDeploy"))

	wsContainer.Add(k8sWS)

	// CI/CD endpoints
	ciWS := new(restful.WebService)
	ciWS.
		Path("/api/ci").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)

	ciWS.Route(ciWS.GET("/ping").To(ciPingHandler).Doc("Ping CI").Operation("ciPing"))
	ciWS.Route(ciWS.POST("/trigger").To(ciTriggerHandler).Doc("Trigger CI Pipeline").Operation("ciTrigger"))

	wsContainer.Add(ciWS)

	// Metrics endpoint (Prometheus)
	if monitoringAdapter != nil {
		if handler, ok := monitoringAdapter.(interface{ MetricsHandler() http.Handler }); ok {
			wsContainer.Handle("/metrics", handler.MetricsHandler())
		}
	}

	// Настройка OpenAPI
	config := restfulspec.Config{
		WebServices:                   wsContainer.RegisteredWebServices(),
		APIPath:                       "/swagger.json",
		PostBuildSwaggerObjectHandler: enrichSwaggerObject,
	}
	wsContainer.Add(restfulspec.NewOpenAPIService(config))

	// Применяем middleware
	handler := LoggingMiddleware(RecoverMiddleware(wsContainer))

	return handler
}

// enrichSwaggerObject обогащает Swagger объект
func enrichSwaggerObject(swo *spec.Swagger) {
	swo.Info = &spec.Info{
		InfoProps: spec.InfoProps{
			Title:       "DevOps Manager API",
			Description: "API для управления DevOps процессами",
			Version:     "1.0.0",
		},
	}
}

// --- Handlers ---

func dockerPingHandler(req *restful.Request, resp *restful.Response) {
	resp.WriteEntity(map[string]string{"status": "docker pong"})
}

func dockerListContainersHandler(req *restful.Request, resp *restful.Response) {
	resp.WriteEntity([]map[string]string{
		{
			"id":     "container1",
			"name":   "test-container-1",
			"status": "running",
		},
		{
			"id":     "container2",
			"name":   "test-container-2",
			"status": "stopped",
		},
	})
}

func dockerPullImageHandler(req *restful.Request, resp *restful.Response) {
	image := req.QueryParameter("image")
	if image == "" {
		resp.WriteErrorString(http.StatusBadRequest, "image parameter is required")
		return
	}

	// Здесь будет реальная логика скачивания образа
	resp.WriteEntity(map[string]string{
		"status": "success",
		"image":  image,
	})
}

func dockerRunContainerHandler(req *restful.Request, resp *restful.Response) {
	var opts ContainerOptions
	err := req.ReadEntity(&opts)
	if err != nil {
		resp.WriteErrorString(http.StatusBadRequest, "invalid request body")
		return
	}

	// Здесь будет реальная логика запуска контейнера
	resp.WriteEntity(map[string]string{
		"status":    "success",
		"container": opts.Name,
	})
}

func k8sPingHandler(req *restful.Request, resp *restful.Response) {
	resp.WriteEntity(map[string]string{"status": "k8s pong"})
}

func k8sDeployHandler(req *restful.Request, resp *restful.Response) {
	namespace := req.QueryParameter("namespace")
	if namespace == "" {
		namespace = "default"
	}

	manifest := req.Request.Body
	if manifest == nil {
		resp.WriteErrorString(http.StatusBadRequest, "manifest is required")
		return
	}

	// Здесь будет реальная логика деплоя в Kubernetes
	resp.WriteEntity(map[string]string{
		"status":    "success",
		"namespace": namespace,
	})
}

func ciPingHandler(req *restful.Request, resp *restful.Response) {
	resp.WriteEntity(map[string]string{"status": "ci pong"})
}

func ciTriggerHandler(req *restful.Request, resp *restful.Response) {
	var trigger struct {
		Project string `json:"project"`
		Ref     string `json:"ref"`
	}
	err := req.ReadEntity(&trigger)
	if err != nil {
		resp.WriteErrorString(http.StatusBadRequest, "invalid request body")
		return
	}

	// Здесь будет реальная логика запуска CI/CD пайплайна
	resp.WriteEntity(map[string]string{
		"status":  "success",
		"project": trigger.Project,
		"ref":     trigger.Ref,
	})
}
