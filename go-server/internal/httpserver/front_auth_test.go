package httpserver

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"aipi-go/internal/auth"
	"aipi-go/internal/config"
)

func TestRequireFrontUserRejectsUserIDWithoutToken(t *testing.T) {
	router := &Router{tokens: auth.NewTokenManager(config.DatabaseConfig{})}
	req := httptest.NewRequest(http.MethodGet, "http://example.test/api/users/user-1/profile", nil)
	_, err := router.requireFrontUser(req, "user-1")
	assertAppErrorStatus(t, err, http.StatusUnauthorized)
}

func TestRequireFrontUserRejectsDifferentTokenOwner(t *testing.T) {
	router := &Router{tokens: auth.NewTokenManager(config.DatabaseConfig{})}
	token, err := router.tokens.CreateUserToken("user-1")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "http://example.test/api/users/user-2/profile", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	_, err = router.requireFrontUser(req, "user-2")
	assertAppErrorStatus(t, err, http.StatusForbidden)
}

func assertAppErrorStatus(t *testing.T, err error, want int) {
	t.Helper()
	var appErr appError
	if !errors.As(err, &appErr) {
		t.Fatalf("error = %v, want appError", err)
	}
	if appErr.status != want {
		t.Fatalf("status = %d, want %d", appErr.status, want)
	}
}
