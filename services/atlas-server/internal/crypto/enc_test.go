package crypto

import "testing"

func TestEncryptDecryptRoundtrip(t *testing.T) {
	secret := DefaultModelSecret
	enc, err := Encrypt("sk-test-secret", secret)
	if err != nil {
		t.Fatal(err)
	}
	if !IsEnc(enc) {
		t.Fatalf("expected ENC(...), got %q", enc)
	}
	plain, err := Decrypt(enc, secret)
	if err != nil {
		t.Fatal(err)
	}
	if plain != "sk-test-secret" {
		t.Fatalf("got %q", plain)
	}
}

func TestDecryptPassthroughPlain(t *testing.T) {
	plain, err := Decrypt("sk-plain", DefaultModelSecret)
	if err != nil {
		t.Fatal(err)
	}
	if plain != "sk-plain" {
		t.Fatalf("got %q", plain)
	}
}

func TestEncryptIdempotentOnEnc(t *testing.T) {
	enc, err := Encrypt("hello", DefaultModelSecret)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Encrypt(enc, DefaultModelSecret)
	if err != nil {
		t.Fatal(err)
	}
	if again != enc {
		t.Fatalf("re-encrypt mutated ENC value")
	}
}

func TestWrongKeyFails(t *testing.T) {
	enc, err := Encrypt("secret", "key-a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decrypt(enc, "key-b"); err == nil {
		t.Fatal("expected decrypt failure")
	}
}
