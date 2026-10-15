package config

import "time"

// SearchOptions controls probe behaviour for a run.
type SearchOptions struct {
	Concurrency    int
	RateLimit      time.Duration // minimum delay between request starts (global)
	DefaultTimeout time.Duration
	DefaultRetries int
	NoFalsePositives bool
}

func DefaultSearchOptions() SearchOptions {
	return SearchOptions{
		Concurrency:    20,
		RateLimit:      0,
		DefaultTimeout: 30 * time.Second,
		DefaultRetries: 2,
	}
}
