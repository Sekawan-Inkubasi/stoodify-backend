package token

import (
	"testing"
)

func TestToken_AccessWithVersion(t *testing.T)	{
	t.Setenv("JWT_SECRET", "super-secret-test-key-minimum-32-chars-long")

	userID := uint(42)
	tokenVersion := 2

	issued, err := GenerateAccess(userID, tokenVersion)
	if err != nil {
		t.Fatalf("gagal generate access token: %v", err)
	}

	parsedUID, jti, parsedVer, err := Parse(issued.Token, TypeAccess)
	if err != nil {
		t.Fatalf("gagal parse access token: %v", err)
	}
	if parsedUID != userID {
		t.Errorf("expected userID %d, got %d", userID, parsedUID)
	}
	if parsedVer != tokenVersion {
		t.Errorf("expected tokenVersion %d, got %d", tokenVersion, parsedVer)
	}
	if jti == "" {
		t.Error("expected non-empty JTI")
	}

	// Refresh token juga mengikat token_version
	refreshIssued, err := GenerateRefresh(userID, tokenVersion)
	if err != nil {
		t.Fatalf("gagal generate refresh token: %v", err)
	}
	rUID, _, rVer, err := Parse(refreshIssued.Token, TypeRefresh)
	if err != nil {
		t.Fatalf("gagal parse refresh token: %v", err)
	}
	if rUID != userID || rVer != tokenVersion {
		t.Errorf("refresh mismatch: uid=%d ver=%d", rUID, rVer)
	}
}

func TestToken_GenerateRandomToken(t *testing.T) {
	raw1, hash1, err1 := GenerateRandomToken()
	if err1 != nil {
		t.Fatalf("generate 1 error: %v", err1)
	}
	raw2, hash2, err2 := GenerateRandomToken()
	if err2 != nil {
		t.Fatalf("generate 2 error: %v", err2)
	}

	if raw1 == raw2 {
		t.Error("dua token acak tidak boleh sama")
	}
	if hash1 != Hash(raw1) {
		t.Error("hash1 tidak cocok dengan sha256(raw1)")
	}
	if hash2 != Hash(raw2) {
		t.Error("hash2 tidak cocok dengan sha256(raw2)")
	}
	if len(raw1) != 64 {
		t.Errorf("expected 64 hex chars (32 bytes), got %d", len(raw1))
	}
}

func TestToken_ParseWrongType(t *testing.T) {
	t.Setenv("JWT_SECRET", "super-secret-test-key-minimum-32-chars-long")

	issued, _ := GenerateAccess(1, 1)
	_, _, _, err := Parse(issued.Token, TypeRefresh)
	if err != ErrInvalid {
		t.Errorf("expected ErrInvalid saat wantType tidak cocok, got %v", err)
	}
}

func TestToken_ParseTampered(t *testing.T) {
	t.Setenv("JWT_SECRET", "super-secret-test-key-minimum-32-chars-long")

	issued, _ := GenerateAccess(1, 1)
	tampered := issued.Token + "evil"
	_, _, _, err := Parse(tampered, TypeAccess)
	if err != ErrInvalid {
		t.Errorf("expected ErrInvalid untuk token dipalsukan, got %v", err)
	}
}
