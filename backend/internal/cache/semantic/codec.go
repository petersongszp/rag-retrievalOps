package semantic

import (
	"encoding/json"
	"fmt"
)

func EncodeEntry(entry *Entry) ([]byte, error) {
	if err := entry.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(entry)
}

func DecodeEntry(data []byte) (*Entry, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("entry payload is empty")
	}
	var entry Entry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}
	if err := entry.Validate(); err != nil {
		return nil, err
	}
	return &entry, nil
}
