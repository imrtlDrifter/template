package utils

import (
	"bytes"
	"encoding/json"

	"github.com/imrtlDrifter/template/internal/domainerrors"
)

func ValidateCreateTripRequiredFields(body []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return err
	}

	requiredFields := []string{
		"user_id",
		"driver_id",
		"start_point",
		"end_point",
		"price",
	}

	for _, field := range requiredFields {
		if err := requireJSONField(fields, field, field); err != nil {
			return err
		}
	}

	for _, pointName := range []string{"start_point", "end_point"} {
		var pointFields map[string]json.RawMessage
		if err := json.Unmarshal(fields[pointName], &pointFields); err != nil {
			return err
		}

		for _, coordinate := range []string{"latitude", "longitude"} {
			fieldPath := pointName + "." + coordinate
			if err := requireJSONField(pointFields, coordinate, fieldPath); err != nil {
				return err
			}
		}
	}

	return nil
}

func requireJSONField(fields map[string]json.RawMessage, name, fieldPath string) error {
	value, exists := fields[name]
	if !exists || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		return &domainerrors.ValidationError{
			Field:   fieldPath,
			Message: "is required",
		}
	}

	return nil
}

func ValidatePositionRequiredFields(body []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return err
	}
	if err := requireJSONField(fields, "recorded_at", "recorded_at"); err != nil {
		return err
	}
	if err := requireJSONField(fields, "coordinates", "coordinates"); err != nil {
		return err
	}

	var coordinates map[string]json.RawMessage
	if err := json.Unmarshal(fields["coordinates"], &coordinates); err != nil {
		return err
	}
	if err := requireJSONField(coordinates, "latitude", "coordinates.latitude"); err != nil {
		return err
	}
	return requireJSONField(coordinates, "longitude", "coordinates.longitude")
}
