package types

import "time"

type Task struct {
	Id      string
	Delay   time.Duration
	Body    []byte
	TakenAt time.Time
}
