/*
|--------------------------------------------------------------------------
| Flush
|--------------------------------------------------------------------------
|
| Flushing removes this store's keys and nothing else: other stores and
| other applications may share the server, so FLUSHDB is never an option.
| In a cluster each node only sees its own keys, so every node is scanned,
| and keys are deleted one by one because keys from different slots cannot
| share a command.
|
*/

package cache

import (
	"context"
	"fmt"

	"github.com/redis/rueidis"
)

/*
scanBatch is how many keys one SCAN call asks each node for.
*/
const scanBatch = 500

/*
Flush removes every key in the store's keyspace.
*/
func (s *Store) Flush(ctx context.Context) error {
	rdb := s.client.Rueidis()

	for _, node := range rdb.Nodes() {
		if err := s.flushNode(ctx, rdb, node); err != nil {
			return err
		}
	}

	return nil
}

func (s *Store) flushNode(ctx context.Context, rdb, node rueidis.Client) error {
	pattern := s.client.Prefix(s.keyspace) + "*"

	for cursor := uint64(0); ; {
		scan := node.B().Scan().Cursor(cursor).Match(pattern).Count(scanBatch).Build()

		entry, err := node.Do(ctx, scan).AsScanEntry()
		if err != nil {
			return fmt.Errorf("flush scan: %w", err)
		}

		if err := unlink(ctx, rdb, entry.Elements); err != nil {
			return err
		}

		if entry.Cursor == 0 {
			return nil
		}

		cursor = entry.Cursor
	}
}

func unlink(ctx context.Context, rdb rueidis.Client, keys []string) error {
	cmds := make(rueidis.Commands, 0, len(keys))
	for _, key := range keys {
		cmds = append(cmds, rdb.B().Unlink().Key(key).Build())
	}

	for _, reply := range rdb.DoMulti(ctx, cmds...) {
		if err := reply.Error(); err != nil {
			return fmt.Errorf("flush unlink: %w", err)
		}
	}

	return nil
}
