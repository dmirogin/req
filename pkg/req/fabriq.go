package req

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/bsm/redislock"
	"github.com/go-redis/redis/v7"
	"github.com/pkg/errors"
)

type Fabriq struct {
	client   *redis.Client
	locker   *redislock.Client
	logger   Logger
	connType int
}

func SetName(name string) func(q *Q) error {
	return func(q *Q) error {
		q.name = name
		return nil
	}
}

func SetTakeTimeout(timeout time.Duration) func(q *Q) error {
	return func(q *Q) error {
		q.takeTimeout = timeout
		return nil
	}
}

func SetTakenValidationPeriod(period time.Duration) func(q *Q) error {
	return func(q *Q) error {
		q.takenValidationPeriod = period
		return nil
	}
}

func (f *Fabriq) Open(ctx context.Context, options ...func(q *Q) error) (*Q, error) {
	q := &Q{
		client:                f.client,
		locker:                f.locker,
		logger:                f.logger,
		takeTimeout:           defaultTakeTimeout,
		takenValidationPeriod: defaultTakenValidationPeriod,
	}

	for _, option := range options {
		if err := option(q); err != nil {
			return nil, errors.Wrap(err, "apply option")
		}
	}

	// Map id to name if present
	if q.name == "" {
		q.name = "default"
	}
	qid, err := q.client.Get(q.name).Result()
	if err != nil && err != redis.Nil {
		return nil, errors.Wrapf(err, "get id for queue %q", q.name)
	}
	if err == redis.Nil {
		q.id = generateQID()
		if err := q.client.Set(q.name, q.id, 0).Err(); err != nil {
			return nil, errors.Wrap(err, "SET queue name")
		}
	} else {
		q.id = qid
	}

	go q.traverseDelayed(ctx)
	go q.validateTaken(ctx, q.takenValidationPeriod)

	return q, nil
}

func (f *Fabriq) MustOpen(ctx context.Context, options ...func(q *Q) error) *Q {
	q, err := f.Open(ctx, options...)
	if err != nil {
		panic(err)
	}
	return q
}

func (f *Fabriq) OpenWithHandler(ctx context.Context, task interface{}, handler HandlerFunc, options ...func(q *Q) error) (*AsynQ, error) {
	q, err := f.Open(ctx, options...)
	if err != nil {
		return nil, errors.Wrap(err, "create queue")
	}
	return NewAsynQ(ctx, q, task, handler), nil
}

func (f *Fabriq) MustOpenWithHandler(ctx context.Context, task interface{}, handler HandlerFunc, options ...func(q *Q) error) *AsynQ {
	aq, err := f.OpenWithHandler(ctx, task, handler)
	if err != nil {
		panic(err)
	}
	return aq
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func generateQID() string {
	return fmt.Sprintf("%d%d", time.Now().Unix(), 100+rand.Intn(900))
}
