package pipeline

type Event struct {
	PID uint32
	SAddr uint32
	DAddr uint32
	SPort uint32
	DPort uint32
	Bytes uint64
	DurationNs uint64
}

var eventPool = sync.Pool{
	New: func() interface{} {
		return &Event{}
	},
}

func AcquireEvent() *Event {
	return eventPool.Get().(*Event)
}

func ReleaseEvent(e *Event) {
	e.PID = 0
	e.SAddr = 0
	e.DAddr = 0
	e.SPort = 0
	e.DPort = 0
	e.Bytes = 0
	e.DurationNs = 0

	eventPool.Put(e)
}
