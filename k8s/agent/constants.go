package agent

import "time"

const (
	requeueSteady  = 60 * time.Second
	requeuePending = 5 * time.Second
	requeueErr     = 15 * time.Second
)
