package main

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/ShimmerGames-Co-Ltd/shimmerdata-go/shimmerdata"
)

func main() {
	client, err := newBatchClient(shimmerdata.SDBatchConfig{
		TempDir:   fmt.Sprintf("../logs/back/app"),
		ServerUrl: "http://localhost:20005",
		//ServerUrl: "https://pt-logtransfer-us.shimmergames.net",
		AppId:     fmt.Sprintf("app-id"),
		AppToken:  fmt.Sprintf("app-token"),
		BatchSize: 47,
		Timeout:   time.Duration(10) * time.Second,
		Compress:  true,
		Interval:  1,
	})
	defer func() {
		closeErr := client.Close()
		if closeErr != nil {
			slog.Error("close error", "error", closeErr)
		}
	}()
	if err != nil {
		panic(err)
	}
	for i := 0; i < 1000; i++ {
		accountID := fmt.Sprintf("%d", i)
		distinctID := fmt.Sprintf("7890123-%d", i)
		err = client.UserSetOnce(accountID, distinctID,
			map[string]interface{}{
				"id":         i,
				"name":       fmt.Sprintf("name-%d", i),
				"age":        i,
				"firstLogin": time.Now(),
			})
		if err != nil {
			slog.Error("UserSetOnce error", "error", err)
			break
		}

		err = client.UserSet(accountID, distinctID,
			map[string]interface{}{
				"money":     i,
				"lastLogin": time.Now(),
			})
		if err != nil {
			slog.Error("userSet error", "error", err)
			break
		}

		err = client.Track(accountID, distinctID, "event_name",
			map[string]interface{}{
				"a": 1,
				"b": 2,
				"c": 3,
				"d": 4,
				"e": 5,
			},
		)
		if err != nil {
			slog.Error("track error", "error", err)
			break
		}

		time.Sleep(10 * time.Millisecond)
	}
}
