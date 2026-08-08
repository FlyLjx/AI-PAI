package apiaccess

import (
	"strings"
	"testing"

	"aipi-go/internal/database"
)

func TestBuildAdminKeyWhereSupportsServerSideSearch(t *testing.T) {
	previousDialect := database.CurrentDialect()
	database.SetDialect("mysql")
	defer database.SetDialect(string(previousDialect))

	where, args := buildAdminKeyWhere(ListKeysInput{
		Status:  " ACTIVE ",
		Keyword: " Customer@Example.COM ",
	})
	for _, fragment := range []string{
		"api_access_keys.deleted_at IS NULL",
		"api_access_keys.status = ?",
		"LOWER(api_access_keys.id) LIKE ?",
		"LOWER(api_access_keys.user_id) LIKE ?",
		"LOWER(api_access_keys.name) LIKE ?",
		"LOWER(api_access_keys.key_prefix) LIKE ?",
		"LOWER(COALESCE(users.email, '')) LIKE ?",
	} {
		if !strings.Contains(where, fragment) {
			t.Fatalf("where clause omitted %q: %s", fragment, where)
		}
	}
	if len(args) != 6 {
		t.Fatalf("args length = %d, want 6: %#v", len(args), args)
	}
	if args[0] != "active" {
		t.Fatalf("status arg = %#v, want active", args[0])
	}
	for index, arg := range args[1:] {
		if arg != "%customer@example.com%" {
			t.Fatalf("keyword arg %d = %#v, want lowercase search pattern", index, arg)
		}
	}
}

func TestNormalizePageClampsBounds(t *testing.T) {
	page, pageSize, offset := normalizePage(-2, 500)
	if page != 1 || pageSize != 100 || offset != 0 {
		t.Fatalf("normalized page = (%d, %d, %d), want (1, 100, 0)", page, pageSize, offset)
	}

	page, pageSize, offset = normalizePage(3, 0)
	if page != 3 || pageSize != 20 || offset != 40 {
		t.Fatalf("normalized page = (%d, %d, %d), want (3, 20, 40)", page, pageSize, offset)
	}
}

func TestIdentitySearchKeywordClassification(t *testing.T) {
	for _, keyword := range []string{
		"customer@example.com",
		"sk-aipai-abc123",
		"550e8400-e29b-41d4-a716-446655440000",
	} {
		if !isIdentitySearchKeyword(keyword) {
			t.Fatalf("keyword %q should use identity search", keyword)
		}
	}
	for _, keyword := range []string{"dall-e", "prompt text", "needle"} {
		if isIdentitySearchKeyword(keyword) {
			t.Fatalf("keyword %q should use general search", keyword)
		}
	}
}

func TestBuildAdminKeyWhereUsesResolvedIdentityIDs(t *testing.T) {
	where, args := buildAdminKeyWhere(ListKeysInput{
		IdentityOnly: true,
		UserIDs:      []string{"user-1", "user-1"},
		APIKeyIDs:    []string{"key-1"},
	})
	if strings.Contains(where, "LIKE") || !strings.Contains(where, "api_access_keys.user_id IN (?)") || !strings.Contains(where, "api_access_keys.id IN (?)") {
		t.Fatalf("identity search should use ID filters: %s", where)
	}
	if len(args) != 2 || args[0] != "user-1" || args[1] != "key-1" {
		t.Fatalf("identity search args = %#v, want [user-1 key-1]", args)
	}
}
