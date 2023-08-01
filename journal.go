package progrock

import (
	"encoding/json"
	"os"
)

type journalWriter struct {
	f   *os.File
	enc *json.Encoder
}

func CreateJournal(filePath string) (Writer, error) {
	f, err := os.Create(filePath)
	if err != nil {
		return nil, err
	}

	enc := json.NewEncoder(f)

	return journalWriter{
		f:   f,
		enc: enc,
	}, nil
}

func (w journalWriter) WriteStatus(ev *StatusUpdate) error {
	return w.enc.Encode(ev)
}

func (w journalWriter) Close() error {
	return w.f.Close()
}
