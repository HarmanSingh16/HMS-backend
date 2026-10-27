package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/HarmanSingh16/HMS-backend/internal/config"
	"github.com/HarmanSingh16/HMS-backend/internal/db"
	"github.com/HarmanSingh16/HMS-backend/internal/domain"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	querier db.Querier
	cfg     *config.Config
}

func NewAuthService(querier db.Querier, cfg *config.Config) *AuthService {
	return &AuthService{
		querier: querier,
		cfg:     cfg,
	}
}

func (s *AuthService) Login(ctx context.Context, req domain.LoginRequest) (*domain.TokenPair, error) {
	account, err := s.querier.GetAccountByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrInvalidCredentials
		}
		return nil, err
	}

	if account.Status == db.AccountStatusDeactivated {
		return nil, domain.ErrAccountDeactivated
	}

	if account.Status != db.AccountStatusActive || !account.PasswordHash.Valid {
		return nil, domain.ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(account.PasswordHash.String),
		[]byte(req.Password),
	); err != nil {
		return nil, domain.ErrInvalidCredentials
	}

	return s.generateTokenPair(account.ID, account.Email, domain.Role(account.Role))
}

func (s *AuthService) RefreshToken(ctx context.Context, refreshTokenStr string) (*domain.TokenPair, error) {
	claims, err := s.parseToken(refreshTokenStr)
	if err != nil {
		return nil, domain.ErrInvalidToken
	}

	account, err := s.querier.GetAccountByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrAccountNotFound
		}
		return nil, err
	}

	if account.Status != db.AccountStatusActive {
		return nil, domain.ErrAccountDeactivated
	}

	return s.generateTokenPair(account.ID, account.Email, domain.Role(account.Role))
}

func (s *AuthService) VerifyInvite(ctx context.Context, rawToken string) (*domain.InviteVerificationResponse, error) {
	tokenHash := hashToken(rawToken)

	invite, err := s.querier.GetInviteByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrInviteNotFound
		}
		return nil, err
	}

	if invite.ClaimedAt.Valid {
		return nil, domain.ErrInviteAlreadyClaimed
	}

	if time.Now().After(invite.ExpiresAt.Time) {
		return nil, domain.ErrInviteNotFound
	}

	return &domain.InviteVerificationResponse{
		Email: invite.Email,
		Role:  domain.Role(invite.Role),
	}, nil
}

func (s *AuthService) ClaimInvite(ctx context.Context, req domain.ClaimInviteRequest) error {
	tokenHash := hashToken(req.Token)

	invite, err := s.querier.GetInviteByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrInviteNotFound
		}
		return err
	}

	if invite.ClaimedAt.Valid {
		return domain.ErrInviteAlreadyClaimed
	}

	if time.Now().After(invite.ExpiresAt.Time) {
		return domain.ErrInviteNotFound
	}

	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	err = s.querier.ActivateAccountWithPassword(ctx, db.ActivateAccountWithPasswordParams{
		ID: invite.AccountID,
		PasswordHash: pgtype.Text{
			String: string(hashedBytes),
			Valid:  true,
		},
	})

	return s.querier.MarkInviteClaimed(ctx, invite.InviteID)
}

func (s *AuthService) GetMe(ctx context.Context, userID int32) (*domain.MeResponse, error) {
	account, err := s.querier.GetAccountByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrAccountNotFound
		}
		return nil, err
	}

	return &domain.MeResponse{
		ID:    account.ID,
		Email: account.Email,
		Role:  domain.Role(account.Role),
	}, nil
}

// Helper Functions:
func (s *AuthService) generateTokenPair(userId int32, email string, role domain.Role) (*domain.TokenPair, error) {
	now := time.Now()
	accessExpiry := now.Add(s.cfg.AccessTokenTTL)
	refreshExpiry := now.Add(s.cfg.RefreshTokenTTL)

	// access token carries: userid, email, role
	accessClaim := &domain.UserClaims{
		UserID: userId,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(accessExpiry),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "hms-api",
		},
	}

	refreshClaim := &domain.UserClaims{
		UserID: userId,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(refreshExpiry),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "hms-api",
		},
	}

	accessToken, err := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		accessClaim,
	).SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return nil, err
	}

	refreshToken, err := jwt.NewWithClaims(
		jwt.SigningMethodHS256, refreshClaim,
	).SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return nil, err
	}

	return &domain.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.cfg.AccessTokenTTL.Seconds()),
	}, nil
}

func (s *AuthService) parseToken(tokenStr string) (*domain.UserClaims, error) {
	token, err := jwt.ParseWithClaims(
		tokenStr,
		&domain.UserClaims{},
		func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, domain.ErrInvalidToken
			}
			return []byte(s.cfg.JWTSecret), nil
		})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*domain.UserClaims)
	if !ok || !token.Valid {
		return nil, domain.ErrInvalidToken
	}

	return claims, nil
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
