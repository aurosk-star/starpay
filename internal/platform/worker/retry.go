package worker

import (
	"context"
	"time"
)

func WaitUntilReady(ctx context.Context, retryInterval time.Duration, operation func(context.Context) error, onFailure func(error)) error {
	if retryInterval <= 0 {
		retryInterval = time.Second
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := operation(ctx); err == nil {
			return nil
		} else if onFailure != nil {
			onFailure(err)
		}
		timer := time.NewTimer(retryInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}
