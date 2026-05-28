package tokens

import "testing"

func TestNewAndHash(t *testing.T) {
	t.Parallel()

	token, err := New("bb_at_")
	if err != nil {
		t.Fatalf("new token failed: %v", err)
	}
	if len(token) <= len("bb_at_") {
		t.Fatal("token missing random payload")
	}

	hash := Hash(token)
	if len(hash) != 64 {
		t.Fatalf("expected sha256 hex hash length 64, got %d", len(hash))
	}
	if hash == token {
		t.Fatal("hash should not equal token")
	}
}
