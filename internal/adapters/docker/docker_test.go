package docker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestServer(t *testing.T, handler http.Handler) (*httptest.Server, *DockerAdapter) {
	server := httptest.NewServer(handler)

	// Создаем клиент с тестовым URL
	cli, err := client.NewClientWithOpts(
		client.WithHost(server.URL),
		client.WithHTTPClient(server.Client()),
	)
	require.NoError(t, err)

	adapter := &DockerAdapter{
		client: cli,
		ctx:    context.Background(),
	}

	return server, adapter
}

func TestPullImage(t *testing.T) {
	tests := []struct {
		name          string
		image         string
		serverHandler http.HandlerFunc
		wantErr       bool
	}{
		{
			name:  "успешный pull образа",
			image: "test-image",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/v1.41/images/create", r.URL.Path)
				assert.Equal(t, "test-image", r.URL.Query().Get("fromImage"))

				// Имитируем успешный ответ Docker API
				response := map[string]string{
					"status": "Pulling from library/test-image",
				}
				json.NewEncoder(w).Encode(response)
			},
			wantErr: false,
		},
		{
			name:  "ошибка при pull образа",
			image: "invalid-image",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				response := map[string]string{
					"error": "image not found",
				}
				json.NewEncoder(w).Encode(response)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, adapter := setupTestServer(t, tt.serverHandler)
			defer server.Close()

			err := adapter.PullImage(tt.image)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRunContainer(t *testing.T) {
	tests := []struct {
		name          string
		opts          ContainerOptions
		serverHandler http.HandlerFunc
		wantErr       bool
	}{
		{
			name: "успешное создание контейнера",
			opts: ContainerOptions{
				Image: "test-image",
				Name:  "test-container",
				Ports: map[string]string{
					"8080": "80",
				},
				Environment: map[string]string{
					"ENV_VAR": "value",
				},
			},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v1.41/containers/create":
					response := container.ContainerCreateCreatedBody{
						ID: "test-container-id",
					}
					json.NewEncoder(w).Encode(response)
				case "/v1.41/containers/test-container-id/start":
					w.WriteHeader(http.StatusNoContent)
				case "/v1.41/containers/test-container-id/json":
					response := types.ContainerJSON{
						ContainerJSONBase: &types.ContainerJSONBase{
							ID:      "test-container-id",
							Name:    "/test-container",
							Created: strconv.FormatInt(time.Now().Unix(), 10),
							State: &types.ContainerState{
								Status: "running",
							},
						},
						Config: &container.Config{
							Image: "test-image",
						},
					}
					json.NewEncoder(w).Encode(response)
				}
			},
			wantErr: false,
		},
		{
			name: "ошибка при создании контейнера",
			opts: ContainerOptions{
				Image: "invalid-image",
				Name:  "test-container",
			},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				response := map[string]string{
					"message": "No such image: invalid-image",
				}
				json.NewEncoder(w).Encode(response)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, adapter := setupTestServer(t, tt.serverHandler)
			defer server.Close()

			info, err := adapter.RunContainer(tt.opts)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, info)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, info)
				assert.Equal(t, "/test-container", info.Name)
				assert.Equal(t, tt.opts.Image, info.Image)
			}
		})
	}
}

func TestStopAndRemoveContainer(t *testing.T) {
	tests := []struct {
		name          string
		containerID   string
		serverHandler http.HandlerFunc
		wantErr       bool
	}{
		{
			name:        "успешная остановка и удаление контейнера",
			containerID: "test-container-id",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v1.41/containers/test-container-id/stop":
					w.WriteHeader(http.StatusNoContent)
				case "/v1.41/containers/test-container-id":
					w.WriteHeader(http.StatusNoContent)
				}
			},
			wantErr: false,
		},
		{
			name:        "ошибка при остановке несуществующего контейнера",
			containerID: "non-existent",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				response := map[string]string{
					"message": "No such container: non-existent",
				}
				json.NewEncoder(w).Encode(response)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, adapter := setupTestServer(t, tt.serverHandler)
			defer server.Close()

			// Тестируем остановку контейнера
			err := adapter.StopContainer(tt.containerID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Тестируем удаление контейнера
			err = adapter.RemoveContainer(tt.containerID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestListContainers(t *testing.T) {
	tests := []struct {
		name          string
		serverHandler http.HandlerFunc
		wantCount     int
		wantErr       bool
	}{
		{
			name: "успешное получение списка контейнеров",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/v1.41/containers/json", r.URL.Path)
				containers := []types.Container{
					{
						ID:      "container1",
						Names:   []string{"/container1"},
						Image:   "image1",
						Status:  "running",
						Created: time.Now().Unix(),
						State:   "running",
					},
					{
						ID:      "container2",
						Names:   []string{"/container2"},
						Image:   "image2",
						Status:  "exited",
						Created: time.Now().Unix(),
						State:   "exited",
					},
				}
				json.NewEncoder(w).Encode(containers)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name: "ошибка при получении списка контейнеров",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				response := map[string]string{
					"message": "Internal server error",
				}
				json.NewEncoder(w).Encode(response)
			},
			wantCount: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, adapter := setupTestServer(t, tt.serverHandler)
			defer server.Close()

			containers, err := adapter.ListContainers()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, containers)
			} else {
				assert.NoError(t, err)
				assert.Len(t, containers, tt.wantCount)
			}
		})
	}
}
