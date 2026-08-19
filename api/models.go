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
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

const (
	StyleOpenAI = "openai"
	StyleGemini = "gemini"
	ArgusModel  = "argus"
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
	Priority int
}

type Config struct {
	Providers []Provider
	Models    []ModelRoute
	Client    *http.Client
}

type Server struct {
	providers map[string]Provider
	models    []ModelRoute
	client    *http.Client
	mu        sync.Mutex
	keyIndex  map[string]int
}

func New(config Config) (*Server, error) {
	server := &Server{
		providers: make(map[string]Provider),
		models:    make([]ModelRoute, 0, len(config.Models)),
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
		server.models = append(server.models, route)
	}
	sort.SliceStable(server.models, func(i, j int) bool {
		return server.models[i].Priority < server.models[j].Priority
	})
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

	if input.Model != ArgusModel {
		writeAPIError(writer, http.StatusBadRequest, fmt.Sprintf("model must be %q", ArgusModel))
		return
	}

	errors := make([]string, 0, len(s.models))
	for _, route := range s.models {
		provider, ok := s.providers[route.Provider]
		if !ok {
			errors = append(errors, route.Provider+" has no keys")
			continue
		}
		key := s.nextKey(provider)
		internalInput := input
		internalInput.Model = route.Model
		var response schema.ChatCompletionResponse
		var err error
		switch provider.Style {
		case StyleGemini:
			response, _, err = s.callGemini(request, provider, key, internalInput)
		case StyleOpenAI:
			response, _, err = s.callOpenAI(request, provider, key, internalInput)
		}
		if err == nil {
			response.Model = route.Model
			writeJSON(writer, http.StatusOK, response)
			return
		}
		errors = append(errors, fmt.Sprintf("%s/%s: %v", route.Provider, route.Model, err))
	}
	if len(errors) == 0 {
		writeAPIError(writer, http.StatusServiceUnavailable, "no configured models are available")
		return
	}
	writeAPIError(writer, http.StatusServiceUnavailable, "all configured models failed: "+strings.Join(errors, "; "))
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
