package control

import (
	"context"
	"github.com/redis/go-redis/v9"
	"time"
)

// Redis is an auxiliary lease index. SQL remains authoritative when it is lost.
func (s *Server) indexLeases(ctx context.Context) {
	if s.cache == nil {
		return
	}
	var rows []Session
	if s.Store.DB.Where("state <> ?", "CLOSED").Find(&rows).Error != nil {
		return
	}
	timeout, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()
	pipe := s.cache.Pipeline()
	for _, x := range rows {
		ttl := time.Until(x.LeaseUntil)
		if ttl > 0 {
			pipe.Set(timeout, "flowlight:lease:"+x.ID, x.LeaseUntil.Format(time.RFC3339Nano), ttl)
		} else {
			pipe.Del(timeout, "flowlight:lease:"+x.ID)
		}
	}
	_, _ = pipe.Exec(timeout)
}
func (s *Server) ConnectCache(rawURL string) error {
	if rawURL == "" {
		return nil
	}
	opt, e := redis.ParseURL(rawURL)
	if e != nil {
		return e
	}
	opt.DialTimeout = 500 * time.Millisecond
	opt.MaxRetries = 0
	s.cache = redis.NewClient(opt)
	return nil
}
