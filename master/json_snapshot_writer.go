package master

import (
	"encoding/json"
	"os"
	"time"
)

type JsonSnapshotWriter struct {
	filename string
}

func NewJsonSnapshotWriter(filename string) *JsonSnapshotWriter {
	return &JsonSnapshotWriter{
		filename: filename,
	}
}

type JsonSnapshotCountersRow struct {
	Time     string
	Counters map[string]int64
}

func (w *JsonSnapshotWriter) WriteCounters(now time.Time, counters map[string]int64) error {
	row := &JsonSnapshotCountersRow{
		Time:     time.Now().Format(time.RFC3339),
		Counters: counters,
	}
	data, err := json.Marshal(row)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(w.filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	data = append(data, ',')
	data = append(data, '\n')
	f.Write(data)

	return nil
}
