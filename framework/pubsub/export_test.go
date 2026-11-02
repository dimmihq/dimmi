package pubsub

/*
UseShardedChannels switches the bus to the sharded channels it uses on a
cluster. A single Valkey server serves them too, so tests cover that path
without a cluster.
*/
func UseShardedChannels(b *Bus) {
	b.sharded = true
}
