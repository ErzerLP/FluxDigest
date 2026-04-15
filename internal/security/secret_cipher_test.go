package security_test

import (
	"testing"

	"rss-platform/internal/security"
)

const testSecretKey = "0123456789abcdef0123456789abcdef"

func TestSecretCipherEncryptDecryptRoundTrip(t *testing.T) {
	cipher, err := security.NewSecretCipher(testSecretKey)
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}

	ciphertext, err := cipher.EncryptString("super-secret-token")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if ciphertext == "super-secret-token" {
		t.Fatal("expected ciphertext to differ from plaintext")
	}
	if len(ciphertext) < len(security.EncryptedValuePrefix)+1 || ciphertext[:len(security.EncryptedValuePrefix)] != security.EncryptedValuePrefix {
		t.Fatalf("expected %s prefix got %q", security.EncryptedValuePrefix, ciphertext)
	}

	plaintext, err := cipher.DecryptString(ciphertext)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if plaintext != "super-secret-token" {
		t.Fatalf("want plaintext restored got %q", plaintext)
	}
}

func TestHasEncryptedPrefix(t *testing.T) {
	if !security.HasEncryptedPrefix(security.EncryptedValuePrefix + "abc") {
		t.Fatal("want enc prefix recognized")
	}
	if security.HasEncryptedPrefix("plain") {
		t.Fatal("did not expect plaintext recognized as encrypted")
	}
}
