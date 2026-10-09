package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// PRD Stoodify: Access Token 15 menit, Refresh Token 7 hari.
const (
	AccessTTL  = 15 * time.Minute
	RefreshTTL = 7 * 24 * time.Hour

	TypeAccess  = "access"
	TypeRefresh = "refresh"
)

var ErrInvalid = errors.New("token tidak valid")

type Issued struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	JTI       string    `json:"-"`
}

type claims struct {
	UserID uint   `json:"uid"`
	Type   string `json:"typ"`
	jwt.RegisteredClaims
}

// Hash: DB tidak pernah menyimpan refresh token mentah, hanya SHA-256-nya.
func Hash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func secret() []byte { return []byte(os.Getenv("JWT_SECRET")) }

func newJTI() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().String()))
	}
	return hex.EncodeToString(b[:])
}

func sign(userID uint, typ string, ttl time.Duration) (Issued, error) {
	now := time.Now()
	jti := newJTI()
	c := claims{
		UserID: userID,
		Type:   typ,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(secret())
	return Issued{Token: signed, ExpiresAt: now.Add(ttl), JTI: jti}, err
}

func GenerateAccess(userID uint) (Issued, error)  { return sign(userID, TypeAccess, AccessTTL) }
func GenerateRefresh(userID uint) (Issued, error) { return sign(userID, TypeRefresh, RefreshTTL) }

// Parse menolak semua kegagalan dengan satu sentinel error agar pemanggil
// tidak membocorkan alasan penolakan. WithValidMethods memblokir alg=none.
func Parse(tokenStr string, wantType string) (uint, string, error) {
	parsed, err := jwt.ParseWithClaims(tokenStr, &claims{}, func(*jwt.Token) (interface{}, error) {
		return secret(), nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
	if err != nil || !parsed.Valid {
		return 0, "", ErrInvalid
	}
	c, ok := parsed.Claims.(*claims)
	if !ok || c.Type != wantType || c.UserID == 0 {
		return 0, "", ErrInvalid
	}
	return c.UserID, c.ID, nil
}
