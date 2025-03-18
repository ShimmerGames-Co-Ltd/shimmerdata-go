package shimmerdata

import "testing"

func TestNewLogConsumerWithConfig(t *testing.T) {
	c, err := NewLogConsumerWithConfig(SDLogConsumerConfig{
		Directory:      "../logs",
		RotateMode:     RotateHourly,
		FileSize:       100,
		FileNamePrefix: "test",
		ChannelSize:    10,
	})
	if err != nil {
		t.Fatal(err)
	}
	client := New(c)
	for i := 0; i < 10; i++ {
		err = client.Track("123456", "7890123", "event_name", map[string]interface{}{
			"a": 1,
			"b": 2,
			"c": 3,
			"d": 4,
			"e": 5,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	err = client.Flush()
	if err != nil {
		t.Fatal(err)
	}
}
