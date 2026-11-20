// Package googleauth memverifikasi Google ID token (Sign in with Google) tanpa
// mempercayai data apa pun dari client: signature, issuer, audience, masa berlaku,
// dan status verifikasi email semuanya dicek di sini.
package googleauth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Kesalahan yang bisa dibedakan pemanggil (dipetakan ke 401/503 di handler).
var (
	ErrTokenTidakValid  = errors.New("token Google tidak valid")
	ErrTokenKedaluwarsa = errors.New("token Google sudah kedaluwarsa")
	ErrAudienceSalah    = errors.New("token Google bukan untuk aplikasi ini")
	ErrIssuerSalah      = errors.New("token Google bukan dari issuer resmi")
	ErrEmailBelumVerif  = errors.New("email Google belum terverifikasi")
	ErrClientIDKosong   = errors.New("GOOGLE_CLIENT_ID belum diisi")
)

// Claims: data minimal yang dibutuhkan backend dari ID token.
type Claims struct {
	Sub           string
	Email         string
	Name          string
	Picture       string
	EmailVerified bool
	ExpiresAt     time.Time
}

// KeySource menyediakan public key RSA berdasarkan `kid` di header token.
// Dibuat interface supaya bisa diuji tanpa jaringan.
type KeySource interface {
	PublicKey(ctx context.Context, kid string) (*rsa.PublicKey, error)
}

// Verifier memverifikasi ID token Google.
type Verifier struct {
	ClientID string
	Keys     KeySource
	Now      func() time.Time
	Issuers  []string
}

// NewVerifier membuat verifier dengan JWKS resmi Google.
func NewVerifier(clientID string) *Verifier {
	return &Verifier{
		ClientID: strings.TrimSpace(clientID),
		Keys:     NewGoogleJWKS(http.DefaultClient),
		Now:      time.Now,
		Issuers:  []string{"https://accounts.google.com", "accounts.google.com"},
	}
}

// Verify memeriksa ID token dan mengembalikan klaimnya.
func (v *Verifier) Verify(ctx context.Context, raw string) (Claims, error) {
	if v.ClientID == "" {
		return Claims{}, ErrClientIDKosong
	}

	var klaim jwt.MapClaims
	token, err := jwt.ParseWithClaims(raw, &klaim, func(t *jwt.Token) (interface{}, error) {
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, ErrTokenTidakValid
		}
		return v.Keys.PublicKey(ctx, kid)
	}, jwt.WithValidMethods([]string{"RS256"}), jwt.WithTimeFunc(func() time.Time {
		if v.Now != nil {
			return v.Now()
		}
		return time.Now()
	}))
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return Claims{}, ErrTokenKedaluwarsa
		}
		return Claims{}, fmt.Errorf("%w: %v", ErrTokenTidakValid, err)
	}
	if !token.Valid {
		return Claims{}, ErrTokenTidakValid
	}

	iss, _ := klaim["iss"].(string)
	if !v.issuerDikenal(iss) {
		return Claims{}, ErrIssuerSalah
	}
	if !v.audienceCocok(klaim["aud"]) {
		return Claims{}, ErrAudienceSalah
	}

	verified, _ := klaim["email_verified"].(bool)
	if !verified {
		return Claims{}, ErrEmailBelumVerif
	}

	sub, _ := klaim["sub"].(string)
	email, _ := klaim["email"].(string)
	if sub == "" || email == "" {
		return Claims{}, ErrTokenTidakValid
	}

	hasil := Claims{
		Sub:           sub,
		Email:         strings.ToLower(strings.TrimSpace(email)),
		Name:          stringKlaim(klaim, "name"),
		Picture:       stringKlaim(klaim, "picture"),
		EmailVerified: verified,
	}
	if exp, ok := klaim["exp"].(float64); ok {
		hasil.ExpiresAt = time.Unix(int64(exp), 0)
	}
	return hasil, nil
}

func (v *Verifier) issuerDikenal(iss string) bool {
	daftar := v.Issuers
	if len(daftar) == 0 {
		daftar = []string{"https://accounts.google.com", "accounts.google.com"}
	}
	for _, dikenal := range daftar {
		if iss == dikenal {
			return true
		}
	}
	return false
}

func (v *Verifier) audienceCocok(aud interface{}) bool {
	switch nilai := aud.(type) {
	case string:
		return nilai == v.ClientID
	case []interface{}:
		for _, item := range nilai {
			if s, ok := item.(string); ok && s == v.ClientID {
				return true
			}
		}
	}
	return false
}

func stringKlaim(klaim jwt.MapClaims, kunci string) string {
	nilai, _ := klaim[kunci].(string)
	return nilai
}

// ==================== JWKS Google ====================

const googleJWKSURL = "https://www.googleapis.com/oauth2/v3/certs"

// GoogleJWKS mengambil public key dari endpoint JWKS Google dan menyimpannya di
// cache. Key di-refresh otomatis ketika `kid` tidak dikenal (Google merotasi key).
type GoogleJWKS struct {
	URL        string
	Client     *http.Client
	TTL        time.Duration
	mu         sync.Mutex
	keys       map[string]*rsa.PublicKey
	fetchedAt  time.Time
	lastErr    error
	refreshDur time.Duration
}

// NewGoogleJWKS membuat cache JWKS dengan klien HTTP tertentu.
func NewGoogleJWKS(client *http.Client) *GoogleJWKS {
	if client == nil {
		client = http.DefaultClient
	}
	return &GoogleJWKS{
		URL:        googleJWKSURL,
		Client:     client,
		TTL:        time.Hour,
		refreshDur: time.Minute,
	}
}

// PublicKey mengembalikan key untuk `kid`, mengambil ulang JWKS bila perlu.
func (g *GoogleJWKS) PublicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	segar := time.Since(g.fetchedAt) < g.TTL && g.lastErr == nil
	if key, ok := g.keys[kid]; ok && segar {
		return key, nil
	}
	// Hindari membanjiri Google kalau kid asing terus dikirim.
	if g.lastErr != nil && time.Since(g.fetchedAt) < g.refreshDur {
		return nil, fmt.Errorf("jwks google tidak tersedia: %w", g.lastErr)
	}

	if err := g.muat(ctx); err != nil {
		g.lastErr = err
		g.fetchedAt = time.Now()
		if key, ok := g.keys[kid]; ok {
			return key, nil
		}
		return nil, err
	}
	g.lastErr = nil

	key, ok := g.keys[kid]
	if !ok {
		return nil, fmt.Errorf("%w: kid %q tidak dikenal", ErrTokenTidakValid, kid)
	}
	return key, nil
}

type jwkSet struct {
	Keys []struct {
		Kid string `json:"kid"`
		Kty string `json:"kty"`
		Alg string `json:"alg"`
		Use string `json:"use"`
		N   string `json:"n"`
		E   string `json:"e"`
	} `json:"keys"`
}

func (g *GoogleJWKS) muat(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.URL, nil)
	if err != nil {
		return err
	}
	resp, err := g.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks google membalas %d", resp.StatusCode)
	}

	var set jwkSet
	if err := json.NewDecoder(resp.Body).Decode(&set); err != nil {
		return err
	}

	hasil := make(map[string]*rsa.PublicKey, len(set.Keys))
	for _, k := range set.Keys {
		if k.Kty != "RSA" || k.Kid == "" {
			continue
		}
		n, err := base64.RawURLEncoding.DecodeString(k.N)
		if err != nil {
			continue
		}
		e, err := base64.RawURLEncoding.DecodeString(k.E)
		if err != nil {
			continue
		}
		hasil[k.Kid] = &rsa.PublicKey{
			N: new(big.Int).SetBytes(n),
			E: int(new(big.Int).SetBytes(e).Int64()),
		}
	}
	if len(hasil) == 0 {
		return errors.New("jwks google tidak berisi key RSA")
	}

	g.keys = hasil
	g.fetchedAt = time.Now()
	return nil
}
