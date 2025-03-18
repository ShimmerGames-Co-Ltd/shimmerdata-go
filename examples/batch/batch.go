package main

import (
	"github.com/ShimmerGames-Co-Ltd/shimmerdata-go.git/shimmerdata"
)

func newBatchClient(conf shimmerdata.SDBatchConfig) (*shimmerdata.SDAnalytics, error) {
	c, err := shimmerdata.NewBatchConsumer(conf)
	if err != nil {
		return nil, err
	}
	shimmerdata.SetLogLevel(shimmerdata.SDLogLevelDebug)
	client := shimmerdata.New(c)
	return client, nil
}
