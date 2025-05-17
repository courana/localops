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

func NewAPI(dockerAdapter, k8sAdapter, ciAdapter interface{}) http.Handler {
	wsContainer := restful.NewContainer()

	// Docker endpoints
	dockerWS := new(restful.WebService)
	dockerWS.
		Path("/api/docker").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)
	dockerWS.Route(dockerWS.GET("/ping").To(dockerPingHandler).Doc("Ping Docker").Operation("dockerPing"))
	// Добавьте другие docker endpoints здесь
	wsContainer.Add(dockerWS)

	// K8s endpoints
	k8sWS := new(restful.WebService)
	k8sWS.
		Path("/api/k8s").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)
	k8sWS.Route(k8sWS.GET("/ping").To(k8sPingHandler).Doc("Ping K8s").Operation("k8sPing"))
	// Добавьте другие k8s endpoints здесь
	wsContainer.Add(k8sWS)

	// CI/CD endpoints
	ciWS := new(restful.WebService)
	ciWS.
		Path("/api/ci").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)
	ciWS.Route(ciWS.GET("/ping").To(ciPingHandler).Doc("Ping CI").Operation("ciPing"))
	// Добавьте другие ci endpoints здесь
	wsContainer.Add(ciWS)

	// Metrics endpoint (Prometheus)
	wsContainer.Handle("/metrics", http.HandlerFunc(metricsHandler))

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

// --- Handlers-заглушки ---

func dockerPingHandler(req *restful.Request, resp *restful.Response) {
	// Здесь вызываем соответствующий адаптер
	resp.WriteEntity(map[string]string{"status": "docker pong"})
}

func k8sPingHandler(req *restful.Request, resp *restful.Response) {
	// Здесь вызываем соответствующий адаптер
	resp.WriteEntity(map[string]string{"status": "k8s pong"})
}

func ciPingHandler(req *restful.Request, resp *restful.Response) {
	// Здесь вызываем соответствующий адаптер
	resp.WriteEntity(map[string]string{"status": "ci pong"})
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("metrics endpoint"))
}
