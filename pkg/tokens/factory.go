package tokens

import "github.com/fsandov/go-sdk/pkg/cache"

func InitWithCache(c cache.Cache) (Service, CacheManager, error) {
	cm := NewCacheManager(c)
	svc, err := NewLongLivedService(DefaultLongLivedConfig(), WithCache(cm))
	if err != nil {
		return nil, nil, err
	}
	return svc, cm, nil
}

func InitWithRefreshTokens(c cache.Cache) (Service, CacheManager, error) {
	cm := NewCacheManager(c)
	svc, err := NewService(DefaultShortLivedConfig(), WithCache(cm))
	if err != nil {
		return nil, nil, err
	}
	return svc, cm, nil
}
