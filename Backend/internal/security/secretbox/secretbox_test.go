package secretbox

import "testing"

const testKey = "zcSZAHVJniJYGFVutzyqiz8hAWYhoxB83+gy9lJpabM="

func TestBoxRoundTrip(t *testing.T) {
	box, err := New(testKey)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	ciphertext, err := box.EncryptString("unlock-password")
	if err != nil {
		t.Fatalf("EncryptString returned error: %v", err)
	}
	if ciphertext == "unlock-password" {
		t.Fatal("ciphertext must not equal plaintext")
	}
	plaintext, err := box.DecryptString(ciphertext)
	if err != nil {
		t.Fatalf("DecryptString returned error: %v", err)
	}
	if plaintext != "unlock-password" {
		t.Fatalf("plaintext = %q", plaintext)
	}
}

func TestNewRejectsWrongLengthKey(t *testing.T) {
	if _, err := New("AAAA"); err == nil {
		t.Fatal("expected wrong length key to fail")
	}
}
