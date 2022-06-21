package do

import (
	"math/rand"
	"time"
)

type acceptorSettings struct {
	min int
	max int
}

func (a *acceptorSettings) newDeadline() time.Time {
	n := time.Duration(rand.Intn(a.max-a.min) + a.min)
	return time.Now().Add(n * time.Millisecond)
}
