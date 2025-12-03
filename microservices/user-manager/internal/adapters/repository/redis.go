package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/internal/application/core/apperrors"
	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/internal/application/core/domain"
	"github.com/redis/go-redis/v9"
)

type RedisRepository struct {
	client *redis.Client
}

type IdempotencyRecord struct {
	Status    string       `json:"status"`
	Response  *domain.User `json:"response,omitempty"`
	Timestamp time.Time    `json:"timestamp"`
}

func hashIP(ip string) string {
	hash := sha256.Sum256([]byte(ip))
	return hex.EncodeToString(hash[:])
}

func NewRedisRepository(ctx context.Context, addr string, password string, db int) (*RedisRepository, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	err := client.Ping(ctx).Err()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &RedisRepository{
		client: client,
	}, nil
}

func (r *RedisRepository) generateKey(email string) string {
	return fmt.Sprintf("user:%s", email)
}

func (r *RedisRepository) generateIdempotencyKey(clientIP, messageID string) string {
	return fmt.Sprintf("idempotency:%s:%s", hashIP(clientIP), messageID)
}

func (r *RedisRepository) Save(ctx context.Context, user *domain.User) error {
	data, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("failed to encode user: %w", err)
	}

	key := "user:" + user.Email

	success, err := r.client.SetNX(ctx, key, data, 0).Result()
	if err != nil {
		return fmt.Errorf("redis connection failed: %w", err)
	}

	if !success {
		return apperrors.ErrUserAlreadyExists
	}

	return nil
}

func (r *RedisRepository) Get(ctx context.Context, email string) (*domain.User, error) {
	key := r.generateKey(email)

	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, apperrors.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get from redis: %w", err)
	}

	var user domain.User
	err = json.Unmarshal([]byte(val), &user)
	if err != nil {
		return nil, fmt.Errorf("failed to decode user data: %w", err)
	}

	return &user, nil
}

func (r *RedisRepository) Delete(ctx context.Context, email string) error {
	key := r.generateKey(email)

	err := r.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}

func (r *RedisRepository) Exists(ctx context.Context, email string) (bool, error) {
	key := r.generateKey(email)

	count, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check existence: %w", err)
	}

	return count > 0, nil
}

func (r *RedisRepository) CheckIdempotency(ctx context.Context, clientIP string, messageID string) (bool, *domain.User, error) {
	key := r.generateIdempotencyKey(clientIP, messageID)

	val, err := r.client.Get(ctx, key).Result()

	if err == redis.Nil {
		record := IdempotencyRecord{
			Status:    "PROCESSING",
			Timestamp: time.Now(),
		}
		data, err := json.Marshal(record)
		if err != nil {
			return false, nil, fmt.Errorf("failed to encode user: %w", err)
		}

		ok, err := r.client.SetNX(ctx, key, data, 24*time.Hour).Result()
		if err != nil {
			return false, nil, err
		}
		if !ok {
			return false, nil, fmt.Errorf("concurrent request detected")
		}
		return true, nil, nil
	}

	if err != nil {
		return false, nil, err
	}

	var record IdempotencyRecord
	if err := json.Unmarshal([]byte(val), &record); err != nil {
		return false, nil, err
	}

	if record.Status == "PROCESSING" {
		return false, nil, fmt.Errorf("request is currently being processed")
	}

	return false, record.Response, nil
}

func (r *RedisRepository) SaveIdempotencyResponse(ctx context.Context, clientIP string, messageID string, user *domain.User) error {
	key := r.generateIdempotencyKey(clientIP, messageID)

	record := IdempotencyRecord{
		Status:    "COMPLETED",
		Response:  user,
		Timestamp: time.Now(),
	}
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to encode user: %w", err)
	}

	return r.client.Set(ctx, key, data, 24*time.Hour).Err()
}

func (r *RedisRepository) DeleteIdempotencyKey(ctx context.Context, clientIP, messageID string) error {
	key := r.generateIdempotencyKey(clientIP, messageID)
	return r.client.Del(ctx, key).Err()
}
