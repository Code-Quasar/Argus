package api

import (
	"Argus/convert"
	"Argus/schema"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

const (
	StyleOpenAI = "openai"
	StyleGemini = "gemini"
)

type Provider struct {
	Name     string
	Endpoint string
	Style    string
	Keys     []string
}

type ModelRoute struct {
	Model    string
	Provider string
}

type Config struct {
	Providers []Provider
	Models    []ModelRoute
	Client    *http.Client
}

type Server struct {
	providers map[string]Provider
	models    map[string]string
	client    *http.Client
	mu        sync.Mutex
	keyIndex  map[string]int
}

func New(config Config) (*Server, error) {
	if len(config.Providers) == 0 {
		return nil, errors.New("api: no providers configured")
	}

	server := &Server{
		providers: make(map[string]Provider),
		models:    make(map[string]string),
		client:    config.Client,
		keyIndex:  make(map[string]int),
	}
	if server.client == nil {
		server.client = &http.Client{Timeout: 60 * time.Second}
	}

	for _, provider := range config.Providers {
		if provider.Name == "" || provider.Endpoint == "" {
			return nil, errors.New("api: provider name and endpoint are required")
		}
		if provider.Style != StyleOpenAI && provider.Style != StyleGemini {
			return nil, fmt.Errorf("api: unsupported provider style %q", provider.Style)
		}
		if len(provider.Keys) == 0 {
			continue
		}
		provider.Endpoint = strings.TrimRight(provider.Endpoint, "/")
		server.providers[provider.Name] = provider
	}
	for _, route := range config.Models {
		if route.Model == "" || route.Provider == "" {
			return nil, errors.New("api: model and provider are required")
		}
		server.models[route.Model] = route.Provider
	}
	return server, nil
}

func (s *Server) Routes() chi.Router {
	router := chi.NewRouter()
	router.Use(middleware.RequestID, middleware.Recoverer)
	router.Get("/healthz", s.health)
	router.Post("/v1/chat/completions", s.chatCompletions)
	return router
}

func (s *Server) health(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) chatCompletions(writer http.ResponseWriter, request *http.Request) {
	var input schema.ChatCompletionRequest
	decoder := json.NewDecoder(io.LimitReader(request.Body, 4<<20))
	if err := decoder.Decode(&input); err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid JSON request")
		return
	}
	if input.Model == "" || len(input.Messages) == 0 {
		writeAPIError(writer, http.StatusBadRequest, "model and messages are required")
		return
	}
	for _, message := range input.Messages {
		if message.Role == "" || message.Content == "" {
			writeAPIError(writer, http.StatusBadRequest, "each message requires role and content")
			return
		}
	}

	providerName, ok := s.providerForModel(input.Model)
	if !ok {
		writeAPIError(writer, http.StatusNotFound, fmt.Sprintf("no provider configured for model %q", input.Model))
		return
	}
	provider, ok := s.providers[providerName]
	if !ok {
		writeAPIError(writer, http.StatusServiceUnavailable, fmt.Sprintf("provider %q has no available keys", providerName))
		return
	}
	key := s.nextKey(provider)

	var response schema.ChatCompletionResponse
	var status int
	var err error
	switch provider.Style {
	case StyleGemini:
		response, status, err = s.callGemini(request, provider, key, input)
	case StyleOpenAI:
		response, status, err = s.callOpenAI(request, provider, key, input)
	}
	if err != nil {
		writeAPIError(writer, status, err.Error())
		return
	}
	if response.Model == "" {
		response.Model = input.Model
	}
	writeJSON(writer, http.StatusOK, response)
}

func (s *Server) providerForModel(model string) (string, bool) {
	if provider, ok := s.models[model]; ok {
		return provider, true
	}
	for pattern, provider := range s.models {
		if strings.HasPrefix(pattern, "*") && strings.HasSuffix(model, strings.TrimPrefix(pattern, "*")) {
			return provider, true
		}
	}
	return "", false
}

func (s *Server) nextKey(provider Provider) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	index := s.keyIndex[provider.Name] % len(provider.Keys)
	s.keyIndex[provider.Name] = index + 1
	return provider.Keys[index]
}

func (s *Server) callOpenAI(incoming *http.Request, provider Provider, key string, input schema.ChatCompletionRequest) (schema.ChatCompletionResponse, int, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return schema.ChatCompletionResponse{}, http.StatusInternalServerError, err
	}
	return s.callUpstream(incoming, provider.Endpoint+"/chat/completions", key, body, nil)
}

func (s *Server) callGemini(incoming *http.Request, provider Provider, key string, input schema.ChatCompletionRequest) (schema.ChatCompletionResponse, int, error) {
	converter := convert.Registry[StyleGemini]
	native, err := converter.ToNative(input)
	if err != nil {
		return schema.ChatCompletionResponse{}, http.StatusBadRequest, err
	}
	body, err := json.Marshal(native)
	if err != nil {
		return schema.ChatCompletionResponse{}, http.StatusInternalServerError, err
	}
	endpoint := fmt.Sprintf("%s/models/%s:generateContent?key=%s", provider.Endpoint, input.Model, key)
	return s.callUpstream(incoming, endpoint, "", body, converter)
}

func (s *Server) callUpstream(incoming *http.Request, endpoint, key string, body []byte, converter convert.Converter) (schema.ChatCompletionResponse, int, error) {
	upstream, err := http.NewRequestWithContext(incoming.Context(), http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return schema.ChatCompletionResponse{}, http.StatusInternalServerError, err
	}
	upstream.Header.Set("Content-Type", "application/json")
	if key != "" {
		upstream.Header.Set("Authorization", "Bearer "+key)
	}

	response, err := s.client.Do(upstream)
	if err != nil {
		return schema.ChatCompletionResponse{}, http.StatusBadGateway, fmt.Errorf("upstream request failed: %w", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return schema.ChatCompletionResponse{}, http.StatusBadGateway, fmt.Errorf("read upstream response: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return schema.ChatCompletionResponse{}, response.StatusCode, fmt.Errorf("upstream returned %s: %s", response.Status, strings.TrimSpace(string(responseBody)))
	}

	if converter != nil {
		translated, err := converter.FromNative(responseBody)
		if err != nil {
			return schema.ChatCompletionResponse{}, http.StatusBadGateway, err
		}
		return translated, http.StatusOK, nil
	}
	var translated schema.ChatCompletionResponse
	if err := json.Unmarshal(responseBody, &translated); err != nil {
		return schema.ChatCompletionResponse{}, http.StatusBadGateway, fmt.Errorf("decode upstream response: %w", err)
	}
	return translated, http.StatusOK, nil
}

func writeAPIError(writer http.ResponseWriter, status int, message string) {
	writeJSON(writer, status, map[string]any{
		"error": map[string]string{
			"message": message,
			"type":    "api_error",
		},
	})
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
