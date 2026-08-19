package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAICompatibleRoute(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer openai-key" {
			t.Fatalf("authorization = %q", request.Header.Get("Authorization"))
		}
		body, _ := io.ReadAll(request.Body)
		if !strings.Contains(string(body), `"model":"llama-test"`) {
			t.Fatalf("request body = %s", body)
		}
		_, _ = writer.Write([]byte(`{"id":"chat-1","model":"llama-test","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}]}`))
	}))
	defer upstream.Close()

	server, err := New(Config{
		Providers: []Provider{{Name: "groq", Endpoint: upstream.URL + "/v1", Style: StyleOpenAI, Keys: []string{"openai-key"}}},
		Models:    []ModelRoute{{Model: "llama-test", Provider: "groq"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"argus","messages":[{"role":"user","content":"hi"}]}`))
	recorder := httptest.NewRecorder()
	server.Routes().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body)
	}
}

func TestGeminiRoute(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1beta/models/gemini-test:generateContent" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		if request.URL.Query().Get("key") != "gemini-key" {
			t.Fatalf("key = %q", request.URL.Query().Get("key"))
		}
		var body struct {
			Contents []struct {
				Role string `json:"role"`
			} `json:"contents"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.Contents) != 1 || body.Contents[0].Role != "user" {
			t.Fatalf("contents = %+v", body.Contents)
		}
		_, _ = writer.Write([]byte(`{"candidates":[{"index":0,"content":{"role":"model","parts":[{"text":"hello"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":2,"candidatesTokenCount":1}}`))
	}))
	defer upstream.Close()

	server, err := New(Config{
		Providers: []Provider{{Name: "gemini", Endpoint: upstream.URL + "/v1beta", Style: StyleGemini, Keys: []string{"gemini-key"}}},
		Models:    []ModelRoute{{Model: "gemini-test", Provider: "gemini"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"argus","messages":[{"role":"system","content":"be concise"},{"role":"user","content":"hi"}]}`))
	recorder := httptest.NewRecorder()
	server.Routes().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body)
	}
	var response struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Model != "argus" || response.Choices[0].Message.Content != "hello" {
		t.Fatalf("response = %+v", response)
	}
}

func TestArgusRouteFallsBackToNextPriorityModel(t *testing.T) {
	var attempts []string
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		attempts = append(attempts, string(body))
		if strings.Contains(string(body), `"model":"first"`) {
			http.Error(writer, "rate limited", http.StatusTooManyRequests)
			return
		}
		_, _ = writer.Write([]byte(`{"model":"second","choices":[{"index":0,"message":{"role":"assistant","content":"fallback"},"finish_reason":"stop"}]}`))
	}))
	defer upstream.Close()

	server, err := New(Config{
		Providers: []Provider{{Name: "groq", Endpoint: upstream.URL, Style: StyleOpenAI, Keys: []string{"key"}}},
		Models: []ModelRoute{
			{Model: "first", Provider: "groq", Priority: 0},
			{Model: "second", Provider: "groq", Priority: 1},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"argus","messages":[{"role":"user","content":"hi"}]}`))
	recorder := httptest.NewRecorder()
	server.Routes().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body)
	}
	if len(attempts) != 2 || !strings.Contains(attempts[0], `"model":"first"`) || !strings.Contains(attempts[1], `"model":"second"`) {
		t.Fatalf("attempts = %v", attempts)
	}
}
