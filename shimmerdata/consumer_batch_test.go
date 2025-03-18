package shimmerdata

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestNewBatchConsumerWithConfig(t *testing.T) {
	var wg sync.WaitGroup
	for i := 1; i <= 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := NewBatchConsumer(SDBatchConfig{
				TempDir:   fmt.Sprintf("../logs/back/app%d", i),
				ServerUrl: "http://localhost:20005",
				AppId:     fmt.Sprintf("app%d", i),
				AppToken:  fmt.Sprintf("app%d", i),
				BatchSize: 47,
				Timeout:   time.Duration(10) * time.Second,
				Compress:  true,
				Interval:  1,
			})
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()

			SetLogLevel(SDLogLevelDebug)
			client := New(c)
			for i := 0; i < 30000; i++ {
				err = client.Track(fmt.Sprintf("%d", i),
					fmt.Sprintf("7890123-%d", i),
					"event_name",
					map[string]interface{}{
						"a": 1,
						"b": 2,
						"c": 3,
						"d": 4,
						"e": 5,
					},
				)
				if err != nil {
					t.Fatal(err)
				}
				time.Sleep(10 * time.Millisecond)
			}

		}()
	}

	wg.Wait()
}
