package password

import "testing"

func TestHashAndVerify(t *testing.T) {
	hash, err := Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("Hash error: %v", err)
	}

	ok, err := Verify("correct horse battery staple", hash)
	if err != nil {
		t.Fatalf("Verify error: %v", err)
	}
	if !ok {
		t.Fatalf("expected ok=true")
	}

	ok, err = Verify("wrong password", hash)
	if err != nil {
		t.Fatalf("Verify wrong password error: %v", err)
	}
	if ok {
		t.Fatalf("expected ok=false")
	}
}

func TestVerify_InvalidHash(t *testing.T) {
	if _, err := Verify("pw", "not-a-hash"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestHash_EmptyPassword(t *testing.T) {
	if _, err := Hash(""); err == nil {
		t.Fatalf("expected error")
	}
}
