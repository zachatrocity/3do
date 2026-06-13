package auth

import "testing"

func TestPasswordHashAndCheck(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "correct horse battery staple" {
		t.Fatal("password hash must not store the raw password")
	}
	if !CheckPassword(hash, "correct horse battery staple") {
		t.Fatal("expected password to verify")
	}
	if CheckPassword(hash, "wrong horse battery staple") {
		t.Fatal("wrong password should not verify")
	}
}

func TestPasswordMinimumLength(t *testing.T) {
	if _, err := HashPassword("short"); err == nil {
		t.Fatal("expected short password to fail")
	}
}

func TestSessionTokenHashUsesSecret(t *testing.T) {
	token := "session-token"
	first := SessionTokenHash("first-secret", token)
	second := SessionTokenHash("second-secret", token)
	if first == second {
		t.Fatal("session token hash should depend on the configured secret")
	}
}
