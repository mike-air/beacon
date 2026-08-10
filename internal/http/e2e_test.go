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
	pool := testsupport.NewTestPool(t)

	cfg := &config.Config{
		Env:                "development",
		Port:               "0",
		JWTSecret:          "e2e-test-secret",
		JWTTTL:             time.Hour,
		CORSOrigins:        []string{"http://localhost:3000"},
		WorkerPollInterval: time.Second,
		WorkerConcurrency:  1,
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
