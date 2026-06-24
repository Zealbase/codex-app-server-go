package protocol

import (
	"encoding/json"
	"fmt"
)

type Item struct {
	ID      string
	Kind    ItemKind
	Payload json.RawMessage
}

type ThreadItem = Item

func NewItem(kind ItemKind, payload any) (Item, error) {
	if payload == nil {
		return Item{Kind: kind}, nil
	}
	if raw, ok := payload.(json.RawMessage); ok {
		return Item{Kind: kind, Payload: append(json.RawMessage(nil), raw...)}, nil
	}
	if raw, ok := payload.([]byte); ok {
		return Item{Kind: kind, Payload: append(json.RawMessage(nil), raw...)}, nil
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return Item{}, err
	}
	return Item{Kind: kind, Payload: json.RawMessage(b)}, nil
}

func (i Item) MarshalJSON() ([]byte, error) {
	obj := map[string]json.RawMessage{}
	if len(i.Payload) > 0 {
		var payload map[string]json.RawMessage
		if err := json.Unmarshal(i.Payload, &payload); err != nil {
			return nil, fmt.Errorf("protocol: item payload must be an object: %w", err)
		}
		for k, v := range payload {
			obj[k] = v
		}
	}
	if i.ID != "" {
		obj["id"] = MustRawJSON(i.ID)
	}
	obj["type"] = MustRawJSON(i.Kind)
	return json.Marshal(obj)
}

func (i *Item) UnmarshalJSON(data []byte) error {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}

	rawType, ok := obj["type"]
	if !ok {
		return fmt.Errorf("protocol: item is missing type")
	}
	if err := json.Unmarshal(rawType, &i.Kind); err != nil {
		return err
	}

	if rawID, ok := obj["id"]; ok {
		if err := json.Unmarshal(rawID, &i.ID); err != nil {
			return err
		}
		delete(obj, "id")
	}
	delete(obj, "type")

	if len(obj) == 0 {
		i.Payload = nil
		return nil
	}

	payload, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	i.Payload = json.RawMessage(payload)
	return nil
}

func (i Item) Decode(v any) error {
	if len(i.Payload) == 0 {
		return json.Unmarshal([]byte("null"), v)
	}
	return json.Unmarshal(i.Payload, v)
}

func (i Item) PayloadBytes() []byte {
	return append([]byte(nil), i.Payload...)
}
