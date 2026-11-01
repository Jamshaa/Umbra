package brain

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const testKey = "test-key-123"

func testClient(t *testing.T, url string) *Client {
	t.Helper()
	c := New(Config{APIKey: testKey, BaseURL: url, Model: "muse-test"})
	c.BaseBackoff = time.Millisecond
	return c
}

func okBody(t *testing.T, id string, calls string) string {
	t.Helper()
	return `{"id":"` + id + `","output":[` + calls + `],"usage":{"input_tokens":10,"output_tokens":20}}`
}

func callJSON(callID, name, args string) string {
	m, _ := json.Marshal(map[string]string{"call_id": callID, "name": name, "arguments": args})
	var v map[string]any
	_ = json.Unmarshal(m, &v)
	v["type"] = "function_call"
	raw, _ := json.Marshal(v)
	return string(raw)
}

func TestFromEnv(t *testing.T) {
	t.Setenv("MODEL_API_KEY", "")
	if _, err := FromEnv(); err == nil {
		t.Fatal("missing key must fail")
	}
	t.Setenv("MODEL_API_KEY", " k ")
	t.Setenv("MODEL_BASE_URL", "")
	t.Setenv("MODEL_NAME", "")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIKey != "k" || cfg.BaseURL != DefaultBaseURL || cfg.Model != DefaultModel {
		t.Fatalf("bad defaults: %+v", cfg)
	}
	t.Setenv("MODEL_BASE_URL", "http://localhost:9999/")
	t.Setenv("MODEL_NAME", "local-model")
	cfg, err = FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BaseURL != "http://localhost:9999" || cfg.Model != "local-model" {
		t.Fatalf("custom values ignored: %+v", cfg)
	}
}

func TestRequestShape(t *testing.T) {
	var gotMethod, gotAuth, gotCT string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotAuth, gotCT = r.Method, r.Header.Get("Authorization"), r.Header.Get("Content-Type")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Write([]byte(okBody(t, "resp_1", "")))
	}))
	defer srv.Close()

	c := testClient(t, srv.URL)
	if _, err := c.Call(context.Background(), []InputItem{UserMessage("hi")}, nil, ""); err != nil {
		t.Fatal(err)
	}
	if gotMethod != "POST" {
		t.Fatalf("method = %q", gotMethod)
	}
	if gotAuth != "Bearer "+testKey {
		t.Fatalf("bad auth header: %q", gotAuth)
	}
	if gotCT != "application/json" {
		t.Fatalf("content-type = %q", gotCT)
	}
	raw, _ := json.Marshal(gotBody)
	body := string(raw)
	if strings.Contains(body, "reasoning_effort") {
		t.Fatalf("reasoning_effort must never be sent: %s", body)
	}
	if gotBody["tool_choice"] != "auto" {
		t.Fatalf("tool_choice must be auto: %s", body)
	}
	if gotBody["model"] != "muse-test" {
		t.Fatalf("model missing: %s", body)
	}
	if _, ok := gotBody["previous_response_id"]; ok {
		t.Fatalf("previous_response_id must be omitted when empty: %s", body)
	}
}

func TestChainingSendsPreviousID(t *testing.T) {
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var v map[string]any
		_ = json.NewDecoder(r.Body).Decode(&v)
		raw, _ := json.Marshal(v)
		bodies = append(bodies, string(raw))
		w.Write([]byte(okBody(t, "resp_chain", "")))
	}))
	defer srv.Close()

	c := testClient(t, srv.URL)
	if _, err := c.Call(context.Background(), []InputItem{UserMessage("a")}, nil, "resp_prev"); err != nil {
		t.Fatal(err)
	}
	if len(bodies) != 1 || !strings.Contains(bodies[0], `"previous_response_id":"resp_prev"`) {
		t.Fatalf("previous_response_id not chained: %v", bodies)
	}
}

func TestSingleAndMultipleCalls(t *testing.T) {
	two := callJSON("call_1", "get_manifest", `{"kind":"Deployment"}`) + "," +
		callJSON("call_2", "get_logs", `{"tail":20}`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(okBody(t, "resp_2", two)))
	}))
	defer srv.Close()

	res, err := testClient(t, srv.URL).Call(context.Background(), []InputItem{UserMessage("x")}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if res.ResponseID != "resp_2" || len(res.Calls) != 2 {
		t.Fatalf("bad result: %+v", res)
	}
	if res.Calls[0].CallID != "call_1" || res.Calls[1].Name != "get_logs" {
		t.Fatalf("calls misparsed: %+v", res.Calls)
	}
	if res.Calls[0].Arguments != `{"kind":"Deployment"}` {
		t.Fatalf("arguments must stay raw JSON: %q", res.Calls[0].Arguments)
	}
}

func TestNoCallsIsNotAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"id":"resp_0","output":[{"type":"reasoning"},{"type":"message"}],"usage":{"input_tokens":1,"output_tokens":2}}`))
	}))
	defer srv.Close()

	res, err := testClient(t, srv.URL).Call(context.Background(), nil, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Calls) != 0 || res.ResponseID != "resp_0" {
		t.Fatalf("bad result: %+v", res)
	}
}

func TestRetryOn500ThenSuccess(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write([]byte(okBody(t, "resp_ok", "")))
	}))
	defer srv.Close()

	res, err := testClient(t, srv.URL).Call(context.Background(), nil, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if res.ResponseID != "resp_ok" || atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("want 1 retry then success, got calls=%d res=%+v", calls, res)
	}
}

func TestNoRetryOn400AndNoKeyLeak(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer srv.Close()

	_, err := testClient(t, srv.URL).Call(context.Background(), nil, nil, "")
	if err == nil || !strings.Contains(err.Error(), "400") {
		t.Fatalf("want 400 error, got %v", err)
	}
	if strings.Contains(err.Error(), testKey) {
		t.Fatalf("error leaks the key: %v", err)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("400 must not be retried, got %d calls", calls)
	}
}

func TestTimeoutFailsFast(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		time.Sleep(300 * time.Millisecond)
	}))
	defer srv.Close()

	c := testClient(t, srv.URL)
	c.RequestTimeout = 30 * time.Millisecond
	_, err := c.Call(context.Background(), nil, nil, "")
	if err == nil {
		t.Fatal("want timeout error")
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("timeout must not be retried, got %d calls", calls)
	}
}

func TestContextCancel(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		time.Sleep(300 * time.Millisecond)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := testClient(t, srv.URL).Call(ctx, nil, nil, "")
	if err == nil {
		t.Fatal("want cancellation error")
	}
	if n := atomic.LoadInt32(&calls); n > 1 {
		t.Fatalf("cancelled call must not be retried, got %d calls", n)
	}
}

func TestUsageRecorded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(okBody(t, "resp_u", "")))
	}))
	defer srv.Close()

	res, err := testClient(t, srv.URL).Call(context.Background(), nil, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Usage.ResponseID != "resp_u" || res.Usage.Model != "muse-test" ||
		res.Usage.InputTokens != 10 || res.Usage.OutputTokens != 20 || res.Usage.Latency <= 0 {
		t.Fatalf("bad usage: %+v", res.Usage)
	}
}

func TestCacheDedupesIdenticalCalls(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Write([]byte(okBody(t, "resp_c", "")))
	}))
	defer srv.Close()

	c := testClient(t, srv.URL)
	in := []InputItem{UserMessage("same")}
	if _, err := c.Call(context.Background(), in, nil, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Call(context.Background(), in, nil, ""); err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("identical call must hit cache, got %d server calls", calls)
	}
	other := []InputItem{UserMessage("different")}
	if _, err := c.Call(context.Background(), other, nil, ""); err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("different call must hit server, got %d server calls", calls)
	}
}

func TestMalformedBodyFailsFast(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Write([]byte("not json{{"))
	}))
	defer srv.Close()

	if _, err := testClient(t, srv.URL).Call(context.Background(), nil, nil, ""); err == nil {
		t.Fatal("want decode error")
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("garbage body must not be retried, got %d calls", calls)
	}
}
