package user

import "testing"

func TestBcryptHasher(t *testing.T) {
	hasher := NewBcryptHasher(4)
	hash, err := hasher.Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	if hash == "correct horse battery staple" {
		t.Fatal("Hash() returned the plaintext password")
	}
	if err := hasher.Compare(hash, "correct horse battery staple"); err != nil {
		t.Fatalf("Compare() valid password error = %v", err)
	}
	if err := hasher.Compare(hash, "wrong"); err == nil {
		t.Fatal("Compare() accepted an invalid password")
	}
}
