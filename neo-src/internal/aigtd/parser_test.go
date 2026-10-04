package aigtd_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"little-timer/internal/aigtd"
)

func TestValidateDecomposition_OK(t *testing.T) {
	raw := `{"title":"x","notes":"","is_complex":false,"due_date":null,"estimated_minutes":20,"subtasks":[]}`
	res, errs, err := aigtd.ValidateDecomposition(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(errs) > 0 {
		t.Fatalf("errs = %v", errs)
	}
	if res.Title != "x" || res.IsComplex || res.EstimatedMinutes != 20 {
		t.Fatalf("res = %+v", res)
	}
}

func TestValidateDecomposition_ComplexRequiresSubtasks(t *testing.T) {
	raw := `{"title":"x","is_complex":true,"estimated_minutes":60,"subtasks":[]}`
	_, errs, err := aigtd.ValidateDecomposition(raw)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range errs {
		if strings.Contains(strings.ToLower(e), "subtask") {
			found = true
		}
	}
	if !found {
		t.Fatalf("want subtask-related error, got %v", errs)
	}
}

func TestValidateDecomposition_SubtaskOutOfRange(t *testing.T) {
	raw := `{"title":"x","is_complex":true,"estimated_minutes":60,"subtasks":[
		{"title":"a","estimated_minutes":2},
		{"title":"b","estimated_minutes":25}
	]}`
	_, errs, err := aigtd.ValidateDecomposition(raw)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range errs {
		if strings.Contains(strings.ToLower(e), "estimated") || strings.Contains(strings.ToLower(e), "range") {
			found = true
		}
	}
	if !found {
		t.Fatalf("want estimated/range error, got %v", errs)
	}
}

func TestValidateDecomposition_BadJSON(t *testing.T) {
	_, _, err := aigtd.ValidateDecomposition("not json")
	if err == nil {
		t.Fatal("want error for invalid JSON")
	}
}

// mockProvider 按 responses 切片依次返回,记录调用次数。
type mockProvider struct {
	responses []string
	calls     int
}

func (m *mockProvider) Name() string { return "mock" }
func (m *mockProvider) Complete(ctx context.Context, req aigtd.CompletionRequest) (aigtd.CompletionResponse, error) {
	if m.calls >= len(m.responses) {
		return aigtd.CompletionResponse{}, errors.New("mock: out of responses")
	}
	text := m.responses[m.calls]
	m.calls++
	return aigtd.CompletionResponse{Text: text}, nil
}

func TestParseWithRetry_SecondSucceeds(t *testing.T) {
	mock := &mockProvider{
		responses: []string{
			`{not json`,
			`{"title":"ok","is_complex":false,"estimated_minutes":15,"subtasks":[]}`,
		},
	}
	res, err := aigtd.ParseWithRetry(context.Background(), mock, aigtd.CompletionRequest{}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if res.Title != "ok" {
		t.Errorf("title = %q", res.Title)
	}
	if mock.calls != 2 {
		t.Errorf("expected 2 calls, got %d", mock.calls)
	}
}

func TestParseWithRetry_AllFailReturnsError(t *testing.T) {
	mock := &mockProvider{responses: []string{`bad`, `also bad`}}
	_, err := aigtd.ParseWithRetry(context.Background(), mock, aigtd.CompletionRequest{}, 2)
	if err == nil {
		t.Fatal("want error")
	}
	if mock.calls != 2 {
		t.Errorf("expected 2 calls, got %d", mock.calls)
	}
}

func TestParseWithRetry_MaxAttemptsOneNoRetry(t *testing.T) {
	mock := &mockProvider{responses: []string{`bad`}}
	_, err := aigtd.ParseWithRetry(context.Background(), mock, aigtd.CompletionRequest{}, 1)
	if err == nil {
		t.Fatal("want error")
	}
	if mock.calls != 1 {
		t.Errorf("expected 1 call, got %d", mock.calls)
	}
}

func TestSystemPrompt_IncludesDate(t *testing.T) {
	p := aigtd.SystemPrompt("2026-10-04")
	if !strings.Contains(p, "2026-10-04") {
		t.Errorf("prompt missing current date: %s", p)
	}
	if !strings.Contains(p, "JSON") {
		t.Errorf("prompt missing JSON rule")
	}
}
