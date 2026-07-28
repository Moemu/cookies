package identity

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestPasswordHashIsSaltedAndVerifiable(t *testing.T) {
	t.Parallel()
	first, err := hashPassword("123456")
	if err != nil {
		t.Fatal(err)
	}
	second, err := hashPassword("123456")
	if err != nil {
		t.Fatal(err)
	}
	if string(first) == string(second) {
		t.Fatal("password hashes must use independent random salts")
	}
	if !verifyPassword(first, "123456") || verifyPassword(first, "wrong") {
		t.Fatal("password verification accepted the wrong credential")
	}
	if !strings.HasPrefix(string(first), "pbkdf2-sha256$210000$") {
		t.Fatalf("unexpected password hash format: %q", first)
	}
}

func TestPasswordVerificationRejectsMalformedOrWeakEncoding(t *testing.T) {
	t.Parallel()
	for _, value := range [][]byte{
		nil,
		[]byte("pbkdf2-sha256$1$bad$bad"),
		[]byte("bcrypt$210000$bad$bad"),
	} {
		if verifyPassword(value, "123456") {
			t.Fatalf("malformed password hash was accepted: %q", value)
		}
	}
}

func TestSessionCookiesAreHttpOnlyAndSameSiteStrict(t *testing.T) {
	t.Parallel()
	service := PasswordSessionService{Secure: true}
	expires := time.Now().UTC().Add(time.Hour)
	cookie := service.Cookie("opaque-token", expires)
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" {
		t.Fatalf("unsafe session cookie: %#v", cookie)
	}
	expired := service.ExpiredCookie()
	if expired.MaxAge != -1 || expired.Value != "" || !expired.HttpOnly || !expired.Secure {
		t.Fatalf("unsafe expired session cookie: %#v", expired)
	}
}

func TestAdminSessionCanCreateProviderJobs(t *testing.T) {
	t.Parallel()
	actor := contract.ActorContext{Scopes: adminScopes()}
	if !actor.HasScope("provider.job.create") {
		t.Fatal("admin login must include provider.job.create for image and video generation")
	}
}
