package googleauth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const clientIDUji = "1043856692765-q1n1p9ueutn3a7iif5gm3s7081o9koud.apps.googleusercontent.com"

type keyTetap struct{ key *rsa.PrivateKey }

func (k keyTetap) PublicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	return &k.key.PublicKey, nil
}

func buatKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return key
}

func tandaTangan(t *testing.T, key *rsa.PrivateKey, klaim jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, klaim)
	token.Header["kid"] = "kunci-uji"
	raw, err := token.SignedString(key)
	require.NoError(t, err)
	return raw
}

func klaimValid() jwt.MapClaims {
	now := time.Now()
	return jwt.MapClaims{
		"iss":            "https://accounts.google.com",
		"aud":            clientIDUji,
		"sub":            "109876543210987654321",
		"email":          "Siswa@Example.com",
		"email_verified": true,
		"name":           "Siswa Contoh",
		"picture":        "https://lh3.googleusercontent.com/a/foto",
		"iat":            now.Unix(),
		"exp":            now.Add(time.Hour).Unix(),
	}
}

func verifierUji(key *rsa.PrivateKey) *Verifier {
	return &Verifier{
		ClientID: clientIDUji,
		Keys:     keyTetap{key: key},
		Now:      time.Now,
	}
}

func TestVerifyTokenValid(t *testing.T) {
	key := buatKey(t)
	raw := tandaTangan(t, key, klaimValid())

	klaim, err := verifierUji(key).Verify(context.Background(), raw)
	require.NoError(t, err)
	assert.Equal(t, "109876543210987654321", klaim.Sub)
	assert.Equal(t, "siswa@example.com", klaim.Email, "email harus dinormalkan")
	assert.Equal(t, "Siswa Contoh", klaim.Name)
	assert.True(t, klaim.EmailVerified)
}

func TestVerifyMenolakTokenKedaluwarsa(t *testing.T) {
	key := buatKey(t)
	klaim := klaimValid()
	klaim["exp"] = time.Now().Add(-time.Minute).Unix()

	_, err := verifierUji(key).Verify(context.Background(), tandaTangan(t, key, klaim))
	assert.ErrorIs(t, err, ErrTokenKedaluwarsa)
}

func TestVerifyMenolakAudienceLain(t *testing.T) {
	key := buatKey(t)
	klaim := klaimValid()
	klaim["aud"] = "client-id-aplikasi-lain.apps.googleusercontent.com"

	_, err := verifierUji(key).Verify(context.Background(), tandaTangan(t, key, klaim))
	assert.ErrorIs(t, err, ErrAudienceSalah)
}

func TestVerifyMenolakIssuerLain(t *testing.T) {
	key := buatKey(t)
	klaim := klaimValid()
	klaim["iss"] = "https://evil.example.com"

	_, err := verifierUji(key).Verify(context.Background(), tandaTangan(t, key, klaim))
	assert.ErrorIs(t, err, ErrIssuerSalah)
}

func TestVerifyMenolakEmailBelumVerifikasi(t *testing.T) {
	key := buatKey(t)
	klaim := klaimValid()
	klaim["email_verified"] = false

	_, err := verifierUji(key).Verify(context.Background(), tandaTangan(t, key, klaim))
	assert.ErrorIs(t, err, ErrEmailBelumVerif)
}

func TestVerifyMenolakSignaturePalsu(t *testing.T) {
	key := buatKey(t)
	keyLain := buatKey(t)
	raw := tandaTangan(t, keyLain, klaimValid())

	_, err := verifierUji(key).Verify(context.Background(), raw)
	assert.ErrorIs(t, err, ErrTokenTidakValid)
}

func TestVerifyClientIDKosong(t *testing.T) {
	v := &Verifier{ClientID: ""}
	_, err := v.Verify(context.Background(), "apa-saja")
	assert.ErrorIs(t, err, ErrClientIDKosong)
}

// JWKS: memastikan key diambil dari endpoint, di-cache, dan di-refresh saat kid baru.
func TestGoogleJWKSMemuatDanCache(t *testing.T) {
	key := buatKey(t)
	var hit int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit++
		set := map[string]any{"keys": []map[string]string{{
			"kid": "kunci-uji",
			"kty": "RSA",
			"alg": "RS256",
			"use": "sig",
			"n":   base64URL(key.PublicKey.N.Bytes()),
			"e":   base64URL([]byte{1, 0, 1}),
		}}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(set)
	}))
	defer srv.Close()

	jwks := NewGoogleJWKS(srv.Client())
	jwks.URL = srv.URL

	k1, err := jwks.PublicKey(context.Background(), "kunci-uji")
	require.NoError(t, err)
	require.NotNil(t, k1)

	k2, err := jwks.PublicKey(context.Background(), "kunci-uji")
	require.NoError(t, err)
	assert.Equal(t, k1, k2)
	assert.Equal(t, 1, hit, "key kedua harus diambil dari cache, bukan HTTP lagi")

	_, err = jwks.PublicKey(context.Background(), "kid-tidak-ada")
	assert.Error(t, err)
	assert.Equal(t, 2, hit, "kid asing harus memicu refresh JWKS")
}

func base64URL(b []byte) string {
	const tabel = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	var out []byte
	for i := 0; i < len(b); i += 3 {
		var n uint32
		sisa := len(b) - i
		n = uint32(b[i]) << 16
		if sisa > 1 {
			n |= uint32(b[i+1]) << 8
		}
		if sisa > 2 {
			n |= uint32(b[i+2])
		}
		out = append(out, tabel[(n>>18)&63], tabel[(n>>12)&63])
		if sisa > 1 {
			out = append(out, tabel[(n>>6)&63])
		}
		if sisa > 2 {
			out = append(out, tabel[n&63])
		}
	}
	return string(out)
}
