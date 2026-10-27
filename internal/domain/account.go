package domain

import "github.com/golang-jwt/jwt/v5"

// Account authorization level
type Role string

const (
	RoleAdmin        Role = "admin"
	RoleDoctor       Role = "doctor"
	RolePatient      Role = "patient"
	RoleReceptionist Role = "receptionist"
)

// User identity
type Account struct {
	ID           int32  `json:"id"`
	Email        string `json:"email"`
	PasswordHash string `json:"_"`
	Role         Role   `json:"role"`
}

type UserClaims struct {
	UserID int32  `json:"user_id"`
	Email  string `json:"email"`
	Role   Role   `json:"role"`
	jwt.RegisteredClaims
}

// Auth request & Response DTOs

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// access + refresh token pair, access-short lived, refresh-long lived to get access token again
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"` //in seconds
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// sent when a new user sets their password for the first time
type ClaimInviteRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required,min=8"`
}

// When a user opens the invite link for the first time before setting a password
type InviteVerificationResponse struct {
	Email string `json:"email"`
	Role  Role   `json:"role"`
}

type MeResponse struct {
	ID    int32  `json:"id"`
	Email string `json:"email"`
	Role  Role   `json:"role"`
}
