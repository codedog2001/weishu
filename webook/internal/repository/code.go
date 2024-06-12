package repository

import (
	"context"
	"xiaoweishu/webook/internal/repository/cache"
)

var (
	ErrCodeSendTOOMany   = cache.ErrSetCodeTooMany
	ErrCodeVerifyTooMany = cache.ErrCodeVerifyTooManyTimes
)

type CodeRepository interface {
	Store(ctx context.Context, biz string, phone string, code string) error
	Verify(ctx context.Context, biz string, phone string, inputCode string) (bool, error)
}
type CacheCodeRepository struct {
	cache cache.CodeCache
}

func NewCodeRepository(c cache.CodeCache) CodeRepository {
	return &CacheCodeRepository{cache: c}
}
func (repo *CacheCodeRepository) Store(ctx context.Context, biz string, phone string, code string) error {
	return repo.cache.Set(ctx, biz, phone, code)
}
func (repo *CacheCodeRepository) Verify(ctx context.Context, biz string, phone string, inputCode string) (bool, error) {
	return repo.cache.Verify(ctx, biz, phone, inputCode)
}
