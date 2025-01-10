package fuego

import (
	"strconv"
	"time"
)

// Timing is a struct to represent a server timing.
// Used in the Server-Timing header.
type Timing struct {
	Name string
	Desc string
	Dur  time.Duration
}

// String returns a string representation of a Timing, as defined in https://www.w3.org/TR/server-timing/#the-server-timing-header-field
func (t Timing) String() string {
	s := t.Name + ";dur=" + strconv.Itoa(int(t.Dur.Milliseconds()))
	if t.Desc != "" {
		s += ";desc=\"" + t.Desc + "\""
	}
	return s
}
