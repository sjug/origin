package metrics

import (
	"encoding/json"
	"fmt"
	"time"
)

const (
	markerName        string = "cluster_loader_marker"
	testDurationLabel string = "TestDuration"
)

type BaseMetrics struct {
	// To let the 3rd party know that this log entry is important
	// TODO set this up by config file
	Marker string `json:"marker"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	TestDuration
}

type TestDuration struct {
	StartTime time.Time     `json:"startTime,omitempty"`
	Duration  time.Duration `json:"testDuration,omitempty"`
}

func (td TestDuration) MarshalJSON() ([]byte, error) {
	type Alias TestDuration
	return json.Marshal(&struct {
		Alias
		Duration string `json:"testDuration"`
	}{
		Alias:    (Alias)(td),
		Duration: td.Duration.String(),
	})
}

func (td *TestDuration) UnmarshalJSON(b []byte) error {
	var err error
	type Alias TestDuration
	s := &struct {
		Duration string `json:"testDuration"`
		*Alias
	}{
		Alias: (*Alias)(td),
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	td.Duration, err = time.ParseDuration(s.Duration)
	if err != nil {
		return err
	}
	return nil
}

func (bm *BaseMetrics) UnmarshalJSON(data []byte) error {
	//type Alias BaseMetrics
	//tmp := &struct {
	//	TestDuration
	//	*Alias
	//}{
	//	TestDuration: TestDuration{},
	//	Alias:        (*Alias)(bm),
	//}
	fmt.Printf("UnmarshalJSON: Data input: %+v\n", string(data))
	tmp := struct {
		Marker string `json:"marker"`
		Name   string `json:"name"`
		Type   string `json:"type"`
	}{}
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}

	fmt.Printf("UnmarshalJSON: Post unmarshal struct: %+v\n", tmp)

	bm.Marker = tmp.Marker
	bm.Name = tmp.Name
	bm.Type = tmp.Type

	return nil
}

func LogMetrics(metrics []BaseMetrics) error {
	for _, m := range metrics {
		b, err := json.Marshal(m)
		if err != nil {
			return err
		}
		fmt.Println(string(b))
	}
	return nil
}

func NewTestDuration(name string, startTime time.Time, testDuration time.Duration) BaseMetrics {
	return BaseMetrics{
		Marker: markerName,
		Name:   name,
		Type:   testDurationLabel,
		TestDuration: TestDuration{
			StartTime: startTime,
			Duration:  testDuration,
		},
	}
}
