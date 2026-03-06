package ui

import "time"

type intervalOption struct {
	Label     string
	Duration  time.Duration
	Recurring bool
}

var intervalOptions = []intervalOption{
	{Label: "Run Once", Duration: 0, Recurring: false},
	{Label: "Run Every Minute", Duration: 1 * time.Minute, Recurring: true},
	{Label: "Run Every 5 Minutes", Duration: 5 * time.Minute, Recurring: true},
	{Label: "Run Every 15 Minutes", Duration: 15 * time.Minute, Recurring: true},
	{Label: "Run Every 30 Minutes", Duration: 30 * time.Minute, Recurring: true},
	{Label: "Run Every Hour", Duration: 1 * time.Hour, Recurring: true},
	{Label: "Run Every 6 Hours", Duration: 6 * time.Hour, Recurring: true},
	{Label: "Run Every 12 Hours", Duration: 12 * time.Hour, Recurring: true},
	{Label: "Run Every Day", Duration: 24 * time.Hour, Recurring: true},
}
