package synthient

import "time"

// Timestamp is a Unix time as encoded by the API, normally in seconds.
type Timestamp int64

// Synthient documents millisecond timestamps for some Helios feeds. As
// seconds, values this large would be tens of thousands of years away.
const millisecondThreshold = 1e12

// Time returns t as a time.Time in UTC. Values of 1e12 or more are read as
// milliseconds. The zero Timestamp returns the zero time.Time, so absent
// timestamps report IsZero.
func (t Timestamp) Time() time.Time {
	switch {
	case t == 0:
		return time.Time{}
	case t >= millisecondThreshold:
		return time.UnixMilli(int64(t)).UTC()
	default:
		return time.Unix(int64(t), 0).UTC()
	}
}
