package app

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

// TestSecurityBoundary verifies browser security headers and cross-site challenge rejection.
func TestSecurityBoundary(t *testing.T) {
	var (
		server   *fiber.App = New()
		request  *http.Request
		response *http.Response
		policy   string
		err      error
	)

	request = httptest.NewRequest(http.MethodGet, "/api/v1/challenge", http.NoBody)
	if response, err = server.Test(request); err != nil {
		t.Fatalf("request same-site challenge: %v", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("same-site challenge status = %d", response.StatusCode)
	}

	if policy = response.Header.Get("Content-Security-Policy"); !strings.Contains(policy, "default-src 'self'") {
		t.Fatalf("missing restrictive content security policy: %q", policy)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/v1/challenge", http.NoBody)
	request.Header.Set("Sec-Fetch-Site", "cross-site")
	if response, err = server.Test(request); err != nil {
		t.Fatalf("request cross-site challenge: %v", err)
	}
	
	defer response.Body.Close()

	if response.StatusCode != http.StatusForbidden {
		var body []byte

		if body, err = io.ReadAll(response.Body); err != nil {
			t.Fatalf("read rejection response: %v", err)
		}

		t.Fatalf("cross-site challenge status = %d, body = %s", response.StatusCode, body)
	}
}
