package brain

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultBaseURL is Meta's Responses API endpoint. Override with
	// MODEL_BASE_URL to point at an OpenAI-compatible local server.
	DefaultBaseURL = "https://api.meta.ai/v1"
	// DefaultModel is the agent model for Umbra runs.
	DefaultModel = "muse-spark-1.3"
	// DefaultMaxSteps caps one finding's tool loop (enforced by T2).
	DefaultMaxSteps = 8
	// responsePath is the only path the client ever calls.
	responsePath = "/responses"
	// maxBody caps a response body kept in memory (8 MiB).
	maxBody = 8 << 20
	// maxCacheEntries bounds memory: findings per run are few dozen.
	maxCacheEntries = 512
)

// Config comes from the environment only, never from a file:
// MODEL_API_KEY (required), MODEL_BASE_URL, MODEL_NAME, MODEL_SEED
// (optional; 0 means unset and the server decides).
type Config struct {
	APIKey  string
	BaseURL string
	Model   string
	Seed    int
}

// FromEnv reads the model configuration. It returns an error when
// MODEL_API_KEY is missing; the error never contains the key.
func FromEnv() (Config, error) {
	key := strings.TrimSpace(os.Getenv("MODEL_API_KEY"))
	if key == "" {
		return Config{}, errors.New("brain: MODEL_API_KEY is required (see .env.example)")
	}
	cfg := Config{
		APIKey:  key,
		BaseURL: strings.TrimRight(strings.TrimSpace(os.Getenv("MODEL_BASE_URL")), "/"),
		Model:   strings.TrimSpace(os.Getenv("MODEL_NAME")),
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	// The key travels as an Authorization header on every call: refuse a
	// plaintext non-loopback endpoint rather than leak it on the wire.
	if err := checkBaseURL(cfg.BaseURL); err != nil {
		return Config{}, err
	}
	if s, err := strconv.Atoi(strings.TrimSpace(os.Getenv("MODEL_SEED"))); err == nil {
		cfg.Seed = s
	}
	if cfg.Model == "" {
		cfg.Model = DefaultModel
	}
	return cfg, nil
}

// checkBaseURL refuses plaintext endpoints outside loopback: the API key
// would travel in cleartext on every call. https anywhere and http on
// loopback (local models) stay allowed.
func checkBaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("brain: bad MODEL_BASE_URL %q", raw)
	}
	if strings.EqualFold(u.Scheme, "https") {
		return nil
	}
	if strings.EqualFold(u.Scheme, "http") {
		h := strings.ToLower(u.Hostname())
		if h == "localhost" || h == "127.0.0.1" || h == "::1" {
			return nil
		}
	}
	return fmt.Errorf("brain: refusing non-HTTPS non-loopback MODEL_BASE_URL (key would travel in cleartext)")
}

// Tool is one function the model may call. The caller owns the JSON
// schema; the client passes it through verbatim (flat Responses API
// format: {"type":"function",...} entries in the tools array).
type Tool struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}

// InputItem is one Responses API input element: a plain message
// (Role+Content) or a tool result (Type "function_call_output").
type InputItem struct {
	Type    string `json:"type,omitempty"`
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
	CallID  string `json:"call_id,omitempty"`
	Output  string `json:"output,omitempty"`
}

// UserMessage builds a plain user input item.
func UserMessage(text string) InputItem {
	return InputItem{Role: "user", Content: text}
}

// ToolOutput builds a function_call_output input item.
func ToolOutput(callID, output string) InputItem {
	return InputItem{Type: "function_call_output", CallID: callID, Output: output}
}

// FunctionCall is one tool request from the model. Arguments stays a
// raw JSON string: T2 parses and validates it against the schema,
// because model output is never trusted.
type FunctionCall struct {
	CallID    string
	Name      string
	Arguments string
}

// Usage records one call for the receipt: tokens, latency, response id.
// No credentials, ever.
type Usage struct {
	ResponseID   string
	Model        string
	InputTokens  int
	OutputTokens int
	Latency      time.Duration
}

// StepResult is one Responses API round trip: the calls to execute
// (possibly none) plus the usage to record.
type StepResult struct {
	ResponseID string
	Calls      []FunctionCall
	Usage      Usage
}

// apiResponse mirrors the fields the client reads. Unknown item types
// (reasoning, message) are skipped, never fail the parse.
type apiResponse struct {
	ID     string `json:"id"`
	Output []struct {
		Type      string `json:"type"`
		CallID    string `json:"call_id"`
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"output"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// Client talks to the Responses API. Zero value is not usable;
// build with New. Safe for concurrent use.
type Client struct {
	cfg  Config
	http *http.Client

	// Knobs (T2 reads them when running the loop).
	RequestTimeout time.Duration
	MaxAttempts    int // total tries per Call; 1 disables retries
	BaseBackoff    time.Duration
	MaxSteps       int // loop cap, default DefaultMaxSteps
	// MaxTokensPerFinding caps spend per finding; 0 means no cap.
	MaxTokensPerFinding int
	// NoCache disables the identical-call cache. Benchmarks set it: a
	// sha256 cache would make N repeats return one answer and report
	// fictitious consistency.
	NoCache bool

	mu       sync.Mutex
	cache    map[string]StepResult
	cacheCap int
}

// New builds a client with sane defaults.
func New(cfg Config) *Client {
	return &Client{
		cfg:            cfg,
		http:           &http.Client{},
		RequestTimeout: 120 * time.Second,
		MaxAttempts:    4,
		BaseBackoff:    500 * time.Millisecond,
		MaxSteps:       DefaultMaxSteps,
		cache:          make(map[string]StepResult),
		cacheCap:       maxCacheEntries,
	}
}

// requestBody builds the Responses API payload. tool_choice is always
// "auto": only the tool list constrains the model. reasoning_effort is
// never sent (the model must always reason; "none" returns 400).
func (c *Client) requestBody(model string, input []InputItem, tools []Tool, previousResponseID string, maxOutputTokens int) ([]byte, error) {
	type toolDef struct {
		Type        string          `json:"type"`
		Name        string          `json:"name"`
		Description string          `json:"description,omitempty"`
		Parameters  json.RawMessage `json:"parameters,omitempty"`
	}
	defs := make([]toolDef, 0, len(tools))
	for _, t := range tools {
		defs = append(defs, toolDef{Type: "function", Name: t.Name, Description: t.Description, Parameters: t.Parameters})
	}
	body := map[string]any{
		"model":       model,
		"input":       input,
		"tools":       defs,
		"tool_choice": "auto",
	}
	if c.cfg.Seed != 0 {
		body["seed"] = c.cfg.Seed
	}
	if previousResponseID != "" {
		body["previous_response_id"] = previousResponseID
	}
	if maxOutputTokens > 0 {
		body["max_output_tokens"] = maxOutputTokens
	}
	return json.Marshal(body)
}

// cacheKey hashes everything that makes a request identical, so equal
// calls (same finding context, same turn) never hit the network twice.
func cacheKey(model, prev string, input, tools []byte) string {
	sum := sha256.Sum256([]byte(model + "\n" + prev + "\n" + string(input) + "\n" + string(tools)))
	return hex.EncodeToString(sum[:])
}

func (c *Client) cached(key string) (StepResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	r, ok := c.cache[key]
	return r, ok
}

func (c *Client) store(key string, r StepResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.cache) >= c.cacheCap {
		// Bounded memory: drop one entry when full.
		for k := range c.cache {
			delete(c.cache, k)
			break
		}
	}
	c.cache[key] = r
}

// backoff sleeps base*2^attempt with up to 50% jitter, or returns false
// when ctx ends first.
func backoff(ctx context.Context, base time.Duration, attempt int) bool {
	d := base << attempt
	if d <= 0 || d > 30*time.Second {
		d = 30 * time.Second
	}
	jitter := time.Duration(rand.Int64N(int64(d)/2 + 1))
	t := time.NewTimer(d/2 + jitter)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// transient reports whether err/status deserves a retry: network
// failures, 429 and 5xx. Anything else (400s, parse errors, timeouts,
// cancellations) fails fast so the T2 loop keeps its own budget.
func transient(err error, status int) bool {
	if err != nil {
		return true
	}
	return status == http.StatusTooManyRequests || status >= 500
}

// Call performs one Responses API round trip and parses the turn.
// Identical calls return the cached result without touching the network.
// Errors never contain the API key.
func (c *Client) Call(ctx context.Context, input []InputItem, tools []Tool, previousResponseID string) (StepResult, error) {
	return c.call(ctx, input, tools, previousResponseID, 0)
}

// CallWithBudget performs one request with an optional server-side output cap.
func (c *Client) CallWithBudget(ctx context.Context, input []InputItem, tools []Tool, previousResponseID string, maxOutputTokens int) (StepResult, error) {
	return c.call(ctx, input, tools, previousResponseID, maxOutputTokens)
}

func (c *Client) call(ctx context.Context, input []InputItem, tools []Tool, previousResponseID string, maxOutputTokens int) (StepResult, error) {
	inJSON, err := json.Marshal(input)
	if err != nil {
		return StepResult{}, fmt.Errorf("brain: encode input: %w", err)
	}
	toolsJSON, err := json.Marshal(tools)
	if err != nil {
		return StepResult{}, fmt.Errorf("brain: encode tools: %w", err)
	}
	key := cacheKey(c.cfg.Model, previousResponseID, inJSON, toolsJSON)
	if !c.NoCache {
		if hit, ok := c.cached(key); ok {
			return hit, nil
		}
	}
	body, err := c.requestBody(c.cfg.Model, input, tools, previousResponseID, maxOutputTokens)
	if err != nil {
		return StepResult{}, err
	}
	attempts := c.MaxAttempts
	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 && !backoff(ctx, c.BaseBackoff, attempt-1) {
			return StepResult{}, ctx.Err()
		}
		res, retry, err := c.doOnce(ctx, body)
		if err == nil {
			if !c.NoCache {
				c.store(key, res)
			}
			return res, nil
		}
		lastErr = err
		if !retry || ctx.Err() != nil {
			return StepResult{}, err
		}
	}
	return StepResult{}, lastErr
}

func (c *Client) doOnce(ctx context.Context, body []byte) (StepResult, bool, error) {
	timeout := c.RequestTimeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+responsePath, bytes.NewReader(body))
	if err != nil {
		return StepResult{}, false, fmt.Errorf("brain: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	start := time.Now()
	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			// Timeout or cancellation: fail fast, the T2 loop owns retries.
			return StepResult{}, false, fmt.Errorf("brain: request: %w", ctx.Err())
		}
		return StepResult{}, true, fmt.Errorf("brain: request failed: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return StepResult{}, true, fmt.Errorf("brain: read body: %w", err)
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return StepResult{}, true, fmt.Errorf("brain: transient status %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		snip := strings.TrimSpace(string(data))
		if len(snip) > 300 {
			snip = snip[:300] + "…"
		}
		return StepResult{}, false, fmt.Errorf("brain: status %d: %s", resp.StatusCode, snip)
	}
	var parsed apiResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		return StepResult{}, false, fmt.Errorf("brain: decode response: %w", err)
	}
	out := StepResult{ResponseID: parsed.ID}
	for _, item := range parsed.Output {
		if item.Type != "function_call" {
			continue
		}
		out.Calls = append(out.Calls, FunctionCall{
			CallID:    item.CallID,
			Name:      item.Name,
			Arguments: item.Arguments,
		})
	}
	out.Usage = Usage{
		ResponseID:   parsed.ID,
		Model:        c.cfg.Model,
		InputTokens:  parsed.Usage.InputTokens,
		OutputTokens: parsed.Usage.OutputTokens,
		Latency:      time.Since(start),
	}
	return out, false, nil
}
