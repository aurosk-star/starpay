package service

import (
	"context"
	"database/sql"
	"errors"
	"runtime"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/redis/go-redis/v9"

	"payment-gateway/ent"
)

const (
	StatusOK       = "ok"
	StatusDegraded = "degraded"
)

type StreamTarget struct {
	Name   string
	Stream string
	Group  string
}

type Overview struct {
	Database ComponentStatus `json:"database"`
	Redis    ComponentStatus `json:"redis"`
	Queues   []QueueStatus   `json:"queues"`
	Runtime  RuntimeStatus   `json:"runtime"`
}

type ComponentStatus struct {
	Status    string         `json:"status"`
	LatencyMS int64          `json:"latency_ms"`
	LatencyUS int64          `json:"latency_us"`
	Error     string         `json:"error,omitempty"`
	CheckedAt time.Time      `json:"checked_at"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type QueueStatus struct {
	Name      string    `json:"name"`
	Stream    string    `json:"stream"`
	Group     string    `json:"group"`
	Status    string    `json:"status"`
	Length    int64     `json:"length"`
	Pending   int64     `json:"pending"`
	Consumers int64     `json:"consumers"`
	Error     string    `json:"error,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
}

type RuntimeStatus struct {
	Status     string    `json:"status"`
	GoVersion  string    `json:"go_version"`
	GOOS       string    `json:"goos"`
	GOARCH     string    `json:"goarch"`
	Goroutines int       `json:"goroutines"`
	AllocBytes uint64    `json:"alloc_bytes"`
	CheckedAt  time.Time `json:"checked_at"`
}

type Service struct {
	db      *sql.DB
	client  *ent.Client
	redis   *redis.Client
	streams []StreamTarget
	clock   func() time.Time
}

type Option func(*Service)

func New(options ...Option) Service {
	s := Service{clock: time.Now}
	for _, option := range options {
		option(&s)
	}
	return s
}

func WithDB(db *sql.DB) Option {
	return func(s *Service) {
		s.db = db
	}
}

func WithEntDriver(driver dialect.Driver) Option {
	return func(s *Service) {
		if drv, ok := driver.(*entsql.Driver); ok {
			s.db = drv.DB()
		}
	}
}

func WithEntClient(client *ent.Client) Option {
	return func(s *Service) {
		s.client = client
	}
}

func WithRedis(client *redis.Client) Option {
	return func(s *Service) {
		s.redis = client
	}
}

func WithStreams(streams []StreamTarget) Option {
	return func(s *Service) {
		s.streams = streams
	}
}

func WithClock(clock func() time.Time) Option {
	return func(s *Service) {
		if clock != nil {
			s.clock = clock
		}
	}
}

func (s Service) Overview(ctx context.Context) Overview {
	return Overview{
		Database: s.checkDatabase(ctx),
		Redis:    s.checkRedis(ctx),
		Queues:   s.checkQueues(ctx),
		Runtime:  s.runtimeStatus(),
	}
}

func (s Service) checkDatabase(ctx context.Context) ComponentStatus {
	start := time.Now()
	status := ComponentStatus{Status: StatusOK, CheckedAt: s.clock()}
	if s.db == nil {
		if s.client == nil {
			status.Status = StatusDegraded
			status.Error = "database handle is not configured"
			return status
		}
		if _, err := s.client.User.Query().Limit(1).IDs(ctx); err != nil {
			status.Status = StatusDegraded
			status.Error = err.Error()
		}
		setLatency(&status, time.Since(start))
		return status
	}
	if err := s.db.PingContext(ctx); err != nil {
		status.Status = StatusDegraded
		status.Error = err.Error()
	}
	setLatency(&status, time.Since(start))
	return status
}

func (s Service) checkRedis(ctx context.Context) ComponentStatus {
	start := time.Now()
	status := ComponentStatus{Status: StatusOK, CheckedAt: s.clock()}
	if s.redis == nil {
		status.Status = StatusDegraded
		status.Error = "redis client is not configured"
		return status
	}
	if err := s.redis.Ping(ctx).Err(); err != nil {
		status.Status = StatusDegraded
		status.Error = err.Error()
	}
	setLatency(&status, time.Since(start))
	return status
}

func (s Service) checkQueues(ctx context.Context) []QueueStatus {
	items := make([]QueueStatus, 0, len(s.streams))
	for _, stream := range s.streams {
		item := QueueStatus{
			Name:      stream.Name,
			Stream:    stream.Stream,
			Group:     stream.Group,
			Status:    StatusOK,
			CheckedAt: s.clock(),
		}
		if s.redis == nil {
			item.Status = StatusDegraded
			item.Error = "redis client is not configured"
			items = append(items, item)
			continue
		}
		length, err := s.redis.XLen(ctx, stream.Stream).Result()
		if err != nil {
			item.Status = StatusDegraded
			item.Error = err.Error()
			items = append(items, item)
			continue
		}
		item.Length = length
		pending, err := s.redis.XPending(ctx, stream.Stream, stream.Group).Result()
		if err != nil {
			if !isNoGroupError(err) {
				item.Status = StatusDegraded
				item.Error = err.Error()
			}
			items = append(items, item)
			continue
		}
		item.Pending = pending.Count
		item.Consumers = int64(len(pending.Consumers))
		if item.Pending > 0 {
			item.Status = StatusDegraded
		}
		items = append(items, item)
	}
	return items
}

func (s Service) runtimeStatus() RuntimeStatus {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	return RuntimeStatus{
		Status:     StatusOK,
		GoVersion:  runtime.Version(),
		GOOS:       runtime.GOOS,
		GOARCH:     runtime.GOARCH,
		Goroutines: runtime.NumGoroutine(),
		AllocBytes: mem.Alloc,
		CheckedAt:  s.clock(),
	}
}

func isNoGroupError(err error) bool {
	return err != nil && (errors.Is(err, redis.Nil) || contains(err.Error(), "NOGROUP"))
}

func contains(text string, part string) bool {
	for i := 0; i+len(part) <= len(text); i++ {
		if text[i:i+len(part)] == part {
			return true
		}
	}
	return false
}

func setLatency(status *ComponentStatus, latency time.Duration) {
	status.LatencyMS = latency.Milliseconds()
	status.LatencyUS = latency.Microseconds()
}
