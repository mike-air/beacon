// End-to-end HTTP tests: the real router (NewServer(...).Routes()) over an
// httptest.Server, driven the way a client would — signup, login, create org,
// project, task, list — plus the auth (401) and cross-tenant (403/404) guards.
//
// Course mapping: Chapter 40 — e2e tests. ENV-GATED: skips unless
// TEST_DATABASE_URL is set (testsupport instead of testcontainers). No build tag.
package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"beacon/internal/config"
	beaconhttp "beacon/internal/http"
	"beacon/internal/testsupport"
)

// harness bundles the test server and a helper to make JSON requests.
type harness struct {
	t      *testing.T
	server *httptest.Server
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	return newHarnessWith(t, nil)
}

// newHarnessWith builds the same harness but lets a test adjust the config
// before the server is wired — used by the rate-limit test below, which needs
// the production-shaped limits the other tests deliberately raise.
func newHarnessWith(t *testing.T, tweak func(*config.Config)) *harness {
	t.Helper()
	pool := testsupport.NewTestPool(t)

	cfg := &config.Config{
		Env:                "development",
		Port:               "0",
		JWTSecret:          "e2e-test-secret",
		JWTTTL:             time.Hour,
		CORSOrigins:        []string{"http://localhost:3000"},
		WorkerPollInterval: time.Second,
		WorkerConcurrency:  1,

		// Ch 19 — the rate limiter is real in this router, and the test suite
		// looks exactly like abuse: dozens of signups and logins from one IP in
		// under a second. Left at the production defaults (5/min, burst 10) the
		// suite throttles itself and every later test fails with a 429 that has
		// nothing to do with what it was checking.
		//
		// So the limits are raised here rather than switched off. Switching the
		// middleware off would mean the e2e tests exercise a router that does
		// not exist in production; raising the numbers keeps the same code path
		// with a ceiling the tests do not hit. Ch19's own behaviour is proved
		// separately, at the bottom of this file.
		TenantRateLimitRPS:   10_000,
		TenantRateLimitBurst: 10_000,
		AuthRateLimitRPS:     10_000,
		AuthRateLimitBurst:   10_000,
	}
	if tweak != nil {
		tweak(cfg)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	srv, err := beaconhttp.NewServer(cfg, logger, pool)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ts := httptest.NewServer(srv.Routes())
	t.Cleanup(ts.Close)
	return &harness{t: t, server: ts}
}

// do issues a request with an optional bearer token and JSON body, returning the
// status code and the decoded response body as a generic map.
func (h *harness) do(method, path, token string, body any) (int, map[string]any) {
	h.t.Helper()

	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			h.t.Fatalf("marshal body: %v", err)
		}
		rdr = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(context.Background(), method, h.server.URL+path, rdr)
	if err != nil {
		h.t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := h.server.Client().Do(req)
	if err != nil {
		h.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()

	var decoded map[string]any
	raw, _ := io.ReadAll(resp.Body)
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &decoded)
	}
	return resp.StatusCode, decoded
}

func TestE2EFullFlow(t *testing.T) {
	h := newHarness(t)

	// Signup → 201, returns the user.
	status, body := h.do(http.MethodPost, "/v1/auth/signup", "", map[string]any{
		"email":    "ada@example.com",
		"name":     "Ada",
		"password": "correct horse battery staple",
	})
	if status != http.StatusCreated {
		t.Fatalf("signup status = %d, want 201 (body=%v)", status, body)
	}
	if body["id"] == nil {
		t.Fatalf("signup response missing id: %v", body)
	}

	// Login → 200, grab the token.
	status, body = h.do(http.MethodPost, "/v1/auth/login", "", map[string]any{
		"email":    "ada@example.com",
		"password": "correct horse battery staple",
	})
	if status != http.StatusOK {
		t.Fatalf("login status = %d, want 200 (body=%v)", status, body)
	}
	token, _ := body["token"].(string)
	if token == "" {
		t.Fatalf("login response missing token: %v", body)
	}

	// Create org → 201.
	status, body = h.do(http.MethodPost, "/v1/orgs", token, map[string]any{"name": "Acme"})
	if status != http.StatusCreated {
		t.Fatalf("create org status = %d, want 201 (body=%v)", status, body)
	}
	orgID, _ := body["id"].(string)
	if orgID == "" {
		t.Fatalf("create org response missing id: %v", body)
	}

	// Create project → 201.
	status, body = h.do(http.MethodPost, "/v1/orgs/"+orgID+"/projects", token, map[string]any{"name": "Website"})
	if status != http.StatusCreated {
		t.Fatalf("create project status = %d, want 201 (body=%v)", status, body)
	}
	projectID, _ := body["id"].(string)
	if projectID == "" {
		t.Fatalf("create project response missing id: %v", body)
	}

	// Create task → 201.
	taskPath := "/v1/orgs/" + orgID + "/projects/" + projectID + "/tasks"
	status, body = h.do(http.MethodPost, taskPath, token, map[string]any{"title": "Ship it"})
	if status != http.StatusCreated {
		t.Fatalf("create task status = %d, want 201 (body=%v)", status, body)
	}

	// List tasks → 200, the paginated listResponse shape.
	status, body = h.do(http.MethodGet, taskPath, token, nil)
	if status != http.StatusOK {
		t.Fatalf("list tasks status = %d, want 200 (body=%v)", status, body)
	}
	items, ok := body["items"].([]any)
	if !ok {
		t.Fatalf("list response missing items array: %v", body)
	}
	if len(items) != 1 {
		t.Errorf("listed %d tasks, want 1", len(items))
	}
	if _, ok := body["limit"]; !ok {
		t.Error("list response missing limit field")
	}
}

// TestE2ESingleResourceBodies guards against the exact bug that shipped:
// get-project, update-project, get-task and update-task all answered
// 200 with a completely EMPTY body — Content-Length: 0 — while the write
// itself succeeded. A client reading that as JSON gets "Unexpected end of
// JSON input", which is not a BeaconError, so the UI showed "the server did
// not accept the move" for a move the server had already accepted.
//
// The cause: huma.Output structs that declare a `Status int` field must set
// it on every return path. If the field exists but is left at its Go zero
// value, huma uses that 0 as the response status internally and silently
// skips serializing the body — DefaultStatus on the operation registration
// does not save you, because it is only consulted when the struct has NO
// Status field at all. TestE2EFullFlow above never caught this because it
// only exercises list/create, and both were already setting Status
// explicitly. This test exists to exercise every operation that returns an
// existing resource, which is exactly the shape TestE2EFullFlow does not
// cover.
func TestE2ESingleResourceBodies(t *testing.T) {
	h := newHarness(t)

	_, body := h.do(http.MethodPost, "/v1/auth/signup", "", map[string]any{
		"email": "grace@example.com", "name": "Grace", "password": "correct horse battery staple",
	})
	_, body = h.do(http.MethodPost, "/v1/auth/login", "", map[string]any{
		"email": "grace@example.com", "password": "correct horse battery staple",
	})
	token, _ := body["token"].(string)

	_, body = h.do(http.MethodPost, "/v1/orgs", token, map[string]any{"name": "Acme"})
	orgID, _ := body["id"].(string)

	_, body = h.do(http.MethodPost, "/v1/orgs/"+orgID+"/projects", token, map[string]any{"name": "Website"})
	projectID, _ := body["id"].(string)
	projectPath := "/v1/orgs/" + orgID + "/projects/" + projectID

	_, body = h.do(http.MethodPost, projectPath+"/tasks", token, map[string]any{"title": "Ship it"})
	taskID, _ := body["id"].(string)
	taskPath := projectPath + "/tasks/" + taskID

	cases := []struct {
		name       string
		method     string
		path       string
		reqBody    any
		wantStatus int
		wantField  string // a field that only appears on the real resource, not on {}
	}{
		{"get-project", http.MethodGet, projectPath, nil, http.StatusOK, "name"},
		{
			"update-project", http.MethodPatch, projectPath,
			map[string]any{"name": "Website v2"}, http.StatusOK, "name",
		},
		{"get-task", http.MethodGet, taskPath, nil, http.StatusOK, "title"},
		{
			"update-task", http.MethodPatch, taskPath,
			map[string]any{"title": "Ship it", "status": "in_progress", "position": 2000},
			http.StatusOK, "title",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body := h.do(tc.method, tc.path, token, tc.reqBody)
			if status != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body=%v)", status, tc.wantStatus, body)
			}
			if body == nil {
				t.Fatalf("%s: body was empty — the exact regression this test guards against", tc.name)
			}
			if body[tc.wantField] == nil {
				t.Errorf("%s: response missing %q: %v", tc.name, tc.wantField, body)
			}
		})
	}
}

func TestE2ENoTokenIs401(t *testing.T) {
	h := newHarness(t)

	status, body := h.do(http.MethodGet, "/v1/orgs", "", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("no-token status = %d, want 401 (body=%v)", status, body)
	}
	// errorEnvelope shape: { "error": { "code": ..., "message": ... } }
	errObj, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("401 response not an error envelope: %v", body)
	}
	if errObj["code"] != "unauthenticated" {
		t.Errorf("error.code = %v, want unauthenticated", errObj["code"])
	}
}

func TestE2ECrossTenantForbidden(t *testing.T) {
	h := newHarness(t)

	// User A signs up, logs in, creates an org + project.
	signupLogin := func(email string) string {
		h.do(http.MethodPost, "/v1/auth/signup", "", map[string]any{
			"email": email, "name": "U", "password": "correct horse battery staple",
		})
		_, body := h.do(http.MethodPost, "/v1/auth/login", "", map[string]any{
			"email": email, "password": "correct horse battery staple",
		})
		tok, _ := body["token"].(string)
		if tok == "" {
			t.Fatalf("login for %s returned no token", email)
		}
		return tok
	}

	tokenA := signupLogin("a@example.com")
	tokenB := signupLogin("b@example.com")

	_, body := h.do(http.MethodPost, "/v1/orgs", tokenA, map[string]any{"name": "Alpha"})
	orgA, _ := body["id"].(string)
	_, body = h.do(http.MethodPost, "/v1/orgs/"+orgA+"/projects", tokenA, map[string]any{"name": "Secret"})
	projA, _ := body["id"].(string)

	// User B is not a member of org A → requireOrg returns 403 not_member.
	status, eb := h.do(http.MethodGet, "/v1/orgs/"+orgA+"/projects/"+projA, tokenB, nil)
	if status != http.StatusForbidden {
		t.Fatalf("B reaching A's org: status = %d, want 403 (body=%v)", status, eb)
	}
	errObj, ok := eb["error"].(map[string]any)
	if !ok || errObj["code"] != "not_member" {
		t.Errorf("403 envelope = %v, want code not_member", eb)
	}
}

// Chapter 19 — the rate limiter, through the real router.
//
// The limits are set to production shape here (the other tests raise them; see
// newHarness), so this exercises the same middleware the deployed service runs.
func TestE2EAuthRateLimit(t *testing.T) {
	h := newHarnessWith(t, func(cfg *config.Config) {
		cfg.AuthRateLimitRPS = 5.0 / 60.0 // 5 per minute
		cfg.AuthRateLimitBurst = 3        // small, so the test is fast
	})

	body := map[string]any{
		"email":    "burst@example.com",
		"name":     "Burst",
		"password": "correct horse battery staple",
	}

	// The burst is spent first: three requests get through whatever they answer.
	for i := 0; i < 3; i++ {
		status, _ := h.do(http.MethodPost, "/v1/auth/signup", "", body)
		if status == http.StatusTooManyRequests {
			t.Fatalf("request %d was limited while the burst should still have room", i+1)
		}
	}

	// The fourth finds an empty bucket.
	status, resp := h.do(http.MethodPost, "/v1/auth/signup", "", body)
	if status != http.StatusTooManyRequests {
		t.Fatalf("status after the burst = %d, want 429 (body=%v)", status, resp)
	}
	errObj, ok := resp["error"].(map[string]any)
	if !ok || errObj["code"] != "rate_limited" {
		t.Errorf("429 envelope = %v, want code rate_limited", resp)
	}
}

// Chapter 14 — idempotency, through the real router. The four cases the chapter
// names, in order: new key, replay, changed body, and no key at all.
func TestE2EIdempotency(t *testing.T) {
	h := newHarness(t)

	_, _ = h.do(http.MethodPost, "/v1/auth/signup", "", map[string]any{
		"email":    "idem@example.com",
		"name":     "Idem",
		"password": "correct horse battery staple",
	})
	_, loginBody := h.do(http.MethodPost, "/v1/auth/login", "", map[string]any{
		"email":    "idem@example.com",
		"password": "correct horse battery staple",
	})
	token, _ := loginBody["token"].(string)
	if token == "" {
		t.Fatal("login returned no token")
	}
	_, orgBody := h.do(http.MethodPost, "/v1/orgs", token, map[string]any{"name": "Idem Org"})
	orgID, _ := orgBody["id"].(string)

	const key = "beacon-e2e-idempotency-key-01"
	path := "/v1/orgs/" + orgID + "/projects"
	create := map[string]any{"name": "Only Once"}

	// 1. New key: the handler runs.
	status, first := h.doWithHeaders(http.MethodPost, path, token, create,
		map[string]string{"Idempotency-Key": key})
	if status != http.StatusCreated {
		t.Fatalf("first create = %d (body=%v)", status, first)
	}

	// 2. Same key, same body: the stored response comes back, byte for byte.
	status, second := h.doWithHeaders(http.MethodPost, path, token, create,
		map[string]string{"Idempotency-Key": key})
	if status != http.StatusCreated {
		t.Fatalf("replay = %d (body=%v)", status, second)
	}
	if first["id"] != second["id"] {
		t.Errorf("replay returned a different project: %v vs %v", first["id"], second["id"])
	}

	// 3. Same key, different body: refused, because the key is a promise about
	//    one specific request.
	status, third := h.doWithHeaders(http.MethodPost, path, token,
		map[string]any{"name": "Something Else"},
		map[string]string{"Idempotency-Key": key})
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("reused key with a different body = %d (body=%v)", status, third)
	}

	// 4. No key at all: the request runs normally, and the caller carries the
	//    duplicate risk themselves.
	status, _ = h.do(http.MethodPost, path, token, map[string]any{"name": "Unprotected"})
	if status != http.StatusCreated {
		t.Fatalf("create without a key = %d", status)
	}
	status, _ = h.do(http.MethodPost, path, token, map[string]any{"name": "Unprotected"})
	if status != http.StatusCreated {
		t.Fatalf("second create without a key = %d, want another 201", status)
	}

	// Exactly one "Only Once" project exists, which is the whole claim.
	_, list := h.do(http.MethodGet, path, token, nil)
	items, _ := list["items"].([]any)
	count := 0
	for _, it := range items {
		if m, ok := it.(map[string]any); ok && m["name"] == "Only Once" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("found %d projects named \"Only Once\", want exactly 1", count)
	}
}

// doWithHeaders is do() plus arbitrary request headers, for the idempotency and
// caching tests.
func (h *harness) doWithHeaders(method, path, token string, body any, headers map[string]string) (int, map[string]any) {
	h.t.Helper()

	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			h.t.Fatalf("marshal body: %v", err)
		}
		rdr = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, h.server.URL+path, rdr)
	if err != nil {
		h.t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := h.server.Client().Do(req)
	if err != nil {
		h.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()

	var decoded map[string]any
	raw, _ := io.ReadAll(resp.Body)
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &decoded)
	}
	return resp.StatusCode, decoded
}
