package oncevery

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func BenchmarkOnce(b *testing.B) {
	ctx := context.Background()
	once := New(func(ctx context.Context) (int, error) { return 0, nil })
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			once.Pull(ctx)
		}
	})
}

func TestOnce_Pull_Racing(t *testing.T) {
	ctx := context.Background()

	i := 0
	fn := func(context.Context) (int, error) {
		i += 1

		return i, nil
	}

	o := New(fn)

	var wg sync.WaitGroup

	wg.Add(10)

	for j := 0; j < 10; j++ {
		go func() {
			defer wg.Done()

			r, _ := o.Pull(ctx)
			require.Truef(t, r == 1, "r is not 1, its %d", r)
		}()
	}

	wg.Wait()

	require.Truef(t, i == 1, "i is not 1, its %d", i)
}

func TestOnce_Pull_RetryOnError(t *testing.T) {
	ctx := context.Background()

	i := 0
	fn := func(context.Context) (int, error) {
		i += 1

		return i, errors.New("some error")
	}

	o := New(fn, OptionRetryOnError(true))

	var wg sync.WaitGroup

	wg.Add(10)

	for j := 0; j < 10; j++ {
		go func(j int) {
			defer wg.Done()
			_, _ = o.Pull(ctx)
		}(j)
	}

	wg.Wait()

	require.Truef(t, i == 10, "i is not 10, its %d", i)
}

func TestOnce_Pull_Every(t *testing.T) {
	ctx := context.Background()

	i := 0
	fn := func(context.Context) (int, error) {
		i++
		return i, nil
	}

	o := New(fn, OptionSetEvery(200*time.Millisecond))

	for j := 0; j < 10; j++ {
		_, _ = o.Pull(ctx)
		time.Sleep(100 * time.Millisecond)
	}

	require.Truef(t, i == 5, "i is not 5, its %d", i)
}
