package brew

import "gorm.io/gorm"

// SoftwaresEnqueueFunc queues brew actions onto the Softwares install queue.
// Returns how many items were newly queued and a queue snapshot (any).
type SoftwaresEnqueueFunc func(db *gorm.DB, action string, names []string, kind string) (queued int, snapshot any, err error)

var softwaresEnqueue SoftwaresEnqueueFunc

// SetSoftwaresQueueEnqueue wires brew actions into the Softwares install queue
// (avoids import cycles; called from modules bootstrap).
func SetSoftwaresQueueEnqueue(fn SoftwaresEnqueueFunc) {
	softwaresEnqueue = fn
}
