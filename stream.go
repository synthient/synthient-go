package synthient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
)

const maxEventSize = 16 << 20

// Stream returns an iterator over the feed's real-time events. Each range
// opens a new connection, which the server closes after up to 30 minutes;
// that close ends the iteration without an error. Any other failure is
// yielded once and ends the iteration. Breaking out of the loop closes the
// connection. Events sent between connections are missed, so range again
// to reconnect.
func (f *Feed[T]) Stream(ctx context.Context) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		var zero T

		resp, err := f.client.send(ctx, request{
			method: http.MethodGet,
			path:   f.path + "/stream",
			accept: "application/x-ndjson",
		})
		if err != nil {
			yield(zero, err)
			return
		}
		if resp.StatusCode != http.StatusOK {
			defer closeBody(resp)
			yield(zero, newAPIError(resp))
			return
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64<<10), maxEventSize)
		for scanner.Scan() {
			line := bytes.TrimSpace(scanner.Bytes())
			if len(line) == 0 {
				continue
			}
			var event T
			err := json.Unmarshal(line, &event)
			if err != nil {
				yield(zero, fmt.Errorf("synthient: decode stream event: %w", err))
				return
			}
			if !yield(event, nil) {
				return
			}
		}

		err = scanner.Err()
		if err == nil {
			return
		}
		ctxErr := ctx.Err()
		if ctxErr != nil {
			err = ctxErr
		}
		yield(zero, fmt.Errorf("synthient: read stream: %w", err))
	}
}
