package aigtd_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"little-timer/internal/aigtd"
)

func TestOpenAICompatProvider_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("auth = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"{\"x\":1}"}}],"usage":{"prompt_tokens":7,"completion_tokens":3}}`)
	}))
	defer srv.Close()

	p := aigtd.NewOpenAICompatProvider(srv.URL+"/v1", "test-key", "m", 5*time.Second)
	resp, err := p.Complete(context.Background(), aigtd.CompletionRequest{
		SystemPrompt: "s", UserPrompt: "u", Model: "m", MaxTokens: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != `{"x":1}` {
		t.Errorf("text = %q", resp.Text)
	}
	if resp.InputTokens != 7 || resp.OutputTokens != 3 {
		t.Errorf("tokens = %+v", resp)
	}
	if p.Name() != "openai_compat" {
		t.Errorf("name = %q", p.Name())
	}
}

func TestOpenAICompatProvider_5xxIsRetryable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", 500)
	}))
	defer srv.Close()
	p := aigtd.NewOpenAICompatProvider(srv.URL+"/v1", "k", "m", time.Second)
	_, err := p.Complete(context.Background(), aigtd.CompletionRequest{UserPrompt: "u"})
	if !errors.Is(err, aigtd.ErrRetryable) {
		t.Fatalf("want ErrRetryable, got %v", err)
	}
}

func TestOpenAICompatProvider_4xxNotRetryable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad key", 401)
	}))
	defer srv.Close()
	p := aigtd.NewOpenAICompatProvider(srv.URL+"/v1", "k", "m", time.Second)
	_, err := p.Complete(context.Background(), aigtd.CompletionRequest{UserPrompt: "u"})
	if errors.Is(err, aigtd.ErrRetryable) {
		t.Fatal("4xx must not be retryable")
	}
	if err == nil {
		t.Fatal("want error for 401")
	}
}

func TestAnthropicProvider_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "ak" {
			t.Errorf("api key header missing")
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Errorf("version header missing")
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"content":[{"text":"{\"y\":2}"}],"usage":{"input_tokens":4,"output_tokens":5}}`)
	}))
	defer srv.Close()

	p := aigtd.NewAnthropicProviderWithBaseURL("ak", "claude-3-5-sonnet-latest", 5*time.Second, srv.URL)
	resp, err := p.Complete(context.Background(), aigtd.CompletionRequest{SystemPrompt: "s", UserPrompt: "u"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != `{"y":2}` {
		t.Errorf("text = %q", resp.Text)
	}
	if resp.InputTokens != 4 || resp.OutputTokens != 5 {
		t.Errorf("tokens = %+v", resp)
	}
	if p.Name() != "anthropic" {
		t.Errorf("name = %q", p.Name())
	}
}

func TestAnthropicProvider_5xxIsRetryable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "overloaded", 529)
	}))
	defer srv.Close()
	p := aigtd.NewAnthropicProviderWithBaseURL("ak", "m", time.Second, srv.URL)
	_, err := p.Complete(context.Background(), aigtd.CompletionRequest{UserPrompt: "u"})
	if !errors.Is(err, aigtd.ErrRetryable) {
		t.Fatalf("want ErrRetryable, got %v", err)
	}
}
