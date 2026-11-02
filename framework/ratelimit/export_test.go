package ratelimit

/*
Buckets returns how many keys the pacer holds in memory.
*/
func Buckets(l *Local) int {
	l.mu.Lock()
	defer l.mu.Unlock()

	return len(l.buckets)
}
