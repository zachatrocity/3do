package config

import "testing"

func TestValidateForServeRequiresStrongSessionSecret(t *testing.T) {
	for _, secret := range []string{"", "change-me", "too-short"} {
		cfg := Config{SessionSecret: secret}
		if err := cfg.ValidateForServe(); err == nil {
			t.Fatalf("expected %q to be rejected", secret)
		}
	}

	cfg := Config{SessionSecret: "0123456789abcdef0123456789abcdef"}
	if err := cfg.ValidateForServe(); err != nil {
		t.Fatal(err)
	}
}
