package auth

import (
	"context"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/poponyas/GrpcChat/core/domain"
	pb "github.com/poponyas/GrpcChat/gen/grpc/chat_v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const iterations = 600000

var username = regexp.MustCompile(`^[a-z][a-z0-9_]{2,31}$`)

type Service struct {
	pb.UnimplementedAuthServiceServer
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }
func passwordHash(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, iterations, 32)
	return hex.EncodeToString(salt) + ":" + hex.EncodeToString(key), err
}
func checkPassword(password, encoded string) bool {
	parts := strings.Split(encoded, ":")
	if len(parts) != 2 {
		return false
	}
	salt, err := hex.DecodeString(parts[0])
	if err != nil || len(salt) != 16 {
		return false
	}
	expected, err := hex.DecodeString(parts[1])
	if err != nil || len(expected) != 32 {
		return false
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, iterations, 32)
	return err == nil && subtle.ConstantTimeCompare(key, expected) == 1
}
func tokenHash(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
func (s *Service) session(ctx context.Context, tx pgx.Tx, u domain.UserProfile) (*pb.AuthResponse, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(secret)
	expires := time.Now().UTC().Add(30 * 24 * time.Hour)
	_, err := tx.Exec(ctx, `INSERT INTO chat.sessions(token_hash,user_id,expires_at) VALUES($1,$2,$3)`, tokenHash(token), u.ID, expires)
	if err != nil {
		return nil, err
	}
	return &pb.AuthResponse{Token: token, ExpiresAt: timestamppb.New(expires), Profile: &pb.UserProfile{Id: u.ID.String(), Name: u.Name, Fullname: u.Fullname, CreatedAt: timestamppb.New(u.CreatedAt)}}, nil
}
func (s *Service) Register(ctx context.Context, in *pb.RegisterRequest) (*pb.AuthResponse, error) {
	name := strings.ToLower(strings.TrimSpace(in.GetName()))
	full := strings.TrimSpace(in.GetFullname())
	if !username.MatchString(name) {
		return nil, status.Error(codes.InvalidArgument, "name: 3–32 lowercase Latin letters, digits or _, starting with a letter")
	}
	if full == "" {
		full = name
	}
	if utf8.RuneCountInString(full) > 255 {
		return nil, status.Error(codes.InvalidArgument, "fullname is too long")
	}
	if len(in.GetPassword()) < 8 || len(in.GetPassword()) > 128 {
		return nil, status.Error(codes.InvalidArgument, "password must be 8–128 bytes")
	}
	hash, err := passwordHash(in.GetPassword())
	if err != nil {
		return nil, status.Error(codes.Internal, "cannot hash password")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "database unavailable")
	}
	defer tx.Rollback(ctx)
	u := domain.NewUserProfile(name, full, nil, nil)
	_, err = tx.Exec(ctx, `INSERT INTO chat.users(id,name,fullname,created_at,password_hash) VALUES($1,$2,$3,$4,$5)`, u.ID, u.Name, u.Fullname, u.CreatedAt, hash)
	if err != nil {
		var p *pgconn.PgError
		if errors.As(err, &p) && p.Code == "23505" {
			return nil, status.Error(codes.AlreadyExists, "username is already registered")
		}
		return nil, status.Error(codes.Internal, "cannot create account")
	}
	res, err := s.session(ctx, tx, u)
	if err != nil {
		return nil, status.Error(codes.Internal, "cannot create session")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, status.Error(codes.Internal, "cannot commit account")
	}
	return res, nil
}
func (s *Service) Login(ctx context.Context, in *pb.LoginRequest) (*pb.AuthResponse, error) {
	if len(in.GetPassword()) > 128 {
		return nil, status.Error(codes.InvalidArgument, "password is too long")
	}
	var u domain.UserProfile
	var hash string
	err := s.pool.QueryRow(ctx, `SELECT id,name,fullname,created_at,password_hash FROM chat.users WHERE lower(name)=$1 AND password_hash IS NOT NULL`, strings.ToLower(strings.TrimSpace(in.GetName()))).Scan(&u.ID, &u.Name, &u.Fullname, &u.CreatedAt, &hash)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, status.Error(codes.Internal, "database unavailable")
	}
	if errors.Is(err, pgx.ErrNoRows) { // Perform the same expensive work for an unknown account.
		hash = strings.Repeat("0", 32) + ":" + strings.Repeat("0", 64)
	}
	valid := checkPassword(in.GetPassword(), hash)
	if !valid || err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid username or password")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "database unavailable")
	}
	defer tx.Rollback(ctx)
	res, err := s.session(ctx, tx, u)
	if err != nil {
		return nil, status.Error(codes.Internal, "cannot create session")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, status.Error(codes.Internal, "cannot commit session")
	}
	return res, nil
}
func (s *Service) Authenticate(ctx context.Context, token string) (uuid.UUID, error) {
	if len(token) != 64 {
		return uuid.Nil, status.Error(codes.Unauthenticated, "login required")
	}
	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT user_id FROM chat.sessions WHERE token_hash=$1 AND expires_at>now()`, tokenHash(token)).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, status.Error(codes.Unauthenticated, "session expired or revoked; log in again")
	}
	if err != nil {
		return uuid.Nil, status.Error(codes.Unavailable, "authentication database unavailable")
	}
	return id, nil
}
func (s *Service) Logout(ctx context.Context, in *emptypb.Empty) (*emptypb.Empty, error) {
	_, err := s.pool.Exec(ctx, `DELETE FROM chat.sessions WHERE token_hash=$1`, tokenHash(BearerToken(ctx)))
	if err != nil {
		return nil, status.Error(codes.Internal, "cannot revoke session")
	}
	return &emptypb.Empty{}, nil
}
