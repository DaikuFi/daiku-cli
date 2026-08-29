package api

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Page normalizes the bare-array and common DRF pagination response shapes.
type Page[T any] struct {
	Results  []T
	Count    *int
	Next     *string
	Previous *string
}

func DecodePage[T any](data []byte) (Page[T], error) {
	var items []T
	if err := decodeJSON(data, &items); err == nil {
		return Page[T]{Results: items}, nil
	}
	var envelope struct {
		Results  json.RawMessage `json:"results"`
		Data     json.RawMessage `json:"data"`
		Items    json.RawMessage `json:"items"`
		Count    *int            `json:"count"`
		Next     *string         `json:"next"`
		Previous *string         `json:"previous"`
	}
	if err := decodeJSON(data, &envelope); err != nil {
		return Page[T]{}, fmt.Errorf("decode page: %w", err)
	}
	raw := envelope.Results
	if len(raw) == 0 {
		raw = envelope.Data
	}
	if len(raw) == 0 {
		raw = envelope.Items
	}
	if len(raw) == 0 || string(raw) == "null" {
		return Page[T]{}, fmt.Errorf("decode page: response has no item collection")
	}
	if err := decodeJSON(raw, &items); err != nil {
		return Page[T]{}, fmt.Errorf("decode page items: %w", err)
	}
	return Page[T]{Results: items, Count: envelope.Count, Next: envelope.Next, Previous: envelope.Previous}, nil
}

func decodeJSON(data []byte, out any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	return requireJSONEOF(decoder)
}
