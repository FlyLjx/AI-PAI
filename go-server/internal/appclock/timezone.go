package appclock

import (
	"sync"
	"time"
	_ "time/tzdata"
)

const (
	DefaultLocationName     = "Asia/Shanghai"
	DefaultDatabaseTimeZone = "+08:00"
)

var (
	configureOnce      sync.Once
	configuredLocation *time.Location
)

func ConfigureDefault() *time.Location {
	configureOnce.Do(func() {
		location, err := time.LoadLocation(DefaultLocationName)
		if err != nil {
			location = time.FixedZone("CST", 8*60*60)
		}
		time.Local = location
		configuredLocation = location
	})
	return configuredLocation
}

func DatabaseTime(value time.Time) time.Time {
	if value.IsZero() {
		return value
	}
	location := ConfigureDefault()
	if value.Location() == time.UTC || value.Location().String() == "UTC" {
		return time.Date(
			value.Year(),
			value.Month(),
			value.Day(),
			value.Hour(),
			value.Minute(),
			value.Second(),
			value.Nanosecond(),
			location,
		)
	}
	return value.In(location)
}
