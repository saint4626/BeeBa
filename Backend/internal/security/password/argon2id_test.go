package password

import "testing"

func TestHashAndVerify(t *testing.T) {
	t.Parallel()

	hash, err := Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash failed: %v", err)
	}

	ok, err := Verify("correct horse battery staple", hash)
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if !ok {
		t.Fatal("expected password to verify")
	}

	ok, err = Verify("wrong password", hash)
	if err != nil {
		t.Fatalf("verify wrong password failed: %v", err)
	}
	if ok {
		t.Fatal("expected wrong password to fail")
	}
}
