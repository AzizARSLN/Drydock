package auth

import (
	"errors"
	"time"

	"github.com/AzizARSLN/drydock/internal/models"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// Claims is the JWT payload emitted after a successful login.
type Claims struct {
	UserID   int64  `json:"uid"`
	Username string `json:"usr"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// Service issues and validates authentication tokens.
type Service struct {
	secret []byte
	ttl    time.Duration
}

// NewService builds an auth service bound to the given signing secret and TTL.
func NewService(secret string, ttlHours int) *Service {
	return &Service{secret: []byte(secret), ttl: time.Duration(ttlHours) * time.Hour}
}

// HashPassword derives a bcrypt hash suitable for storage.
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	return string(b), err
}

// VerifyPassword compares a plaintext password against a stored hash.
func VerifyPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// Issue creates a signed JWT for the given user.
func (s *Service) Issue(u *models.User) (string, time.Time, error) {
	exp := time.Now().Add(s.ttl)
	claims := Claims{
		UserID:   u.ID,
		Username: u.Username,
		Role:     u.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.Username,
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(s.secret)
	return signed, exp, err
}

// Parse validates a token string and returns its claims.
func (s *Service) Parse(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	tok, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.secret, nil
	})
	if err != nil || !tok.Valid {
		return nil, errors.New("invalid or expired token")
	}
	return claims, nil
}

// roleRank orders roles from least to most privileged.
var roleRank = map[string]int{
	models.RoleViewer:   1,
	models.RoleOperator: 2,
	models.RoleAdmin:    3,
}

// Can asserts that the caller role satisfies the required minimum role.
func Can(role, required string) bool {
	return roleRank[role] >= roleRank[required]
}

// ValidRole reports whether the given string is a known role.
func ValidRole(role string) bool {
	_, ok := roleRank[role]
	return ok
}
