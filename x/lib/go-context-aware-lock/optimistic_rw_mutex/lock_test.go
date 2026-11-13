package optimisticrwmutex

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func Test_General(t *testing.T) {
	t.Run("version is obsolete", func(t *testing.T) {
		ol := &OptRWMutex{}
		version, obsolete := ol.RLock()
		require.False(t, obsolete)
		require.Equal(t, uint64(0), version)

		ol.Lock()
		require.NotPanics(t, func() {
			ol.Unlock()
		})

		obsolete = ol.RUnlock(version)
		require.True(t, obsolete)
	})

	t.Run("wlock blocks rlock", func(t *testing.T) {
		ol := &OptRWMutex{}

		ol.Lock()

		ch := make(chan uint64, 1)

		go func() {
			// rlock here, and it must be blocked
			version, obsolete := ol.RLock()
			require.False(t, obsolete)
			ch <- version
		}()

		select {
		case <-time.After(200 * time.Millisecond):
		case <-ch:
			require.FailNow(t, "rlock must be blocked")
		}

		ol.Unlock()

		select {
		case <-time.After(200 * time.Millisecond):
			require.FailNow(t, "rlock must be acquired after unlock")
		case version := <-ch:
			require.Equal(t, uint64(0b100), version)
		}
	})

	t.Run("wlock blocks wlock", func(t *testing.T) {
		ol := &OptRWMutex{}

		ol.Lock()

		ch := make(chan bool, 1)

		go func() {
			// rlock here, and it must be blocked
			ol.Lock()
			ch <- true
		}()

		select {
		case <-time.After(200 * time.Millisecond):
		case <-ch:
			require.FailNow(t, "wlock must be blocked")
		}

		ol.Unlock()

		select {
		case <-time.After(200 * time.Millisecond):
			require.FailNow(t, "rlock must be acquired after unlock")
		case version := <-ch:
			require.True(t, version)
		}

		ol.Unlock()
	})

	t.Run("rlock should not block wlock", func(t *testing.T) {
		ol := &OptRWMutex{}

		_, obsolete := ol.RLock()
		require.False(t, obsolete)

		ch := make(chan bool, 1)

		go func() {
			// rlock here, and it must be blocked
			ol.Lock()
			ch <- true
		}()

		select {
		case <-time.After(200 * time.Millisecond):
			require.FailNow(t, "lock must be blocked")
		case signal := <-ch:
			require.True(t, signal)
		}

		ol.Unlock()
	})
}

func Test_Atomicity(t *testing.T) {
	res := 0
	ol := &OptRWMutex{}

	eg, _ := errgroup.WithContext(context.Background())
	eg.SetLimit(20)
	for range 100 {
		eg.Go(func() error {
			for {
				version, _ := ol.RLock()
				upgraded := ol.Upgrade(version)
				if !upgraded {
					continue
				}
				res += 1
				ol.Unlock()

				return nil
			}
		})
	}

	err := eg.Wait()
	require.NoError(t, err)
	require.Equal(t, 100, res)
}
