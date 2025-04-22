package message

import "time"

type deadlineGetter interface {
	GetDeadline() uint64
}

func GetDeadline(m any) (time.Duration, bool) {
	msg, ok := m.(deadlineGetter)
	if !ok {
		return 0, false
	}
	deadline := msg.GetDeadline()
	return time.Duration(deadline), true
}
