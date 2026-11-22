package models

import (
	"github.com/imrtlDrifter/template/internal/domainerrors"

	"github.com/google/uuid"
)

type Check func(*CreateTripCommand) (string, bool, error)

func CheckUserID(c *CreateTripCommand) (string, bool, error) {
	if c.UserID == uuid.Nil {
		return "", false, &domainerrors.ValidationError{
			Field:   "user_id",
			Message: "must be a non-empty UUID",
		}
	}
	return "", false, nil
}

func CheckDriverID(c *CreateTripCommand) (string, bool, error) {
	if c.DriverID == uuid.Nil {
		return "", false, &domainerrors.ValidationError{
			Field:   "driver_id",
			Message: "must be a non-empty UUID",
		}
	}
	return "", false, nil
}

func CheckStartLatitude(c *CreateTripCommand) (string, bool, error) {
	if c.StartPoint.Latitude < -90 || c.StartPoint.Latitude > 90 {
		return "", false, &domainerrors.ValidationError{
			Field:   "start_point.latitude",
			Message: "must be between -90 and 90",
		}
	}
	return "", false, nil
}

func CheckStartLongitude(c *CreateTripCommand) (string, bool, error) {
	if c.StartPoint.Longitude < -180 || c.StartPoint.Longitude > 180 {
		return "", false, &domainerrors.ValidationError{
			Field:   "start_point.longitude",
			Message: "must be between -180 and 180",
		}
	}
	return "", false, nil
}

func CheckEndLatitude(c *CreateTripCommand) (string, bool, error) {
	if c.EndPoint.Latitude < -90 || c.EndPoint.Latitude > 90 {
		return "", false, &domainerrors.ValidationError{
			Field:   "end_point.latitude",
			Message: "must be between -90 and 90",
		}
	}
	return "", false, nil
}

func CheckEndLongitude(c *CreateTripCommand) (string, bool, error) {
	if c.EndPoint.Longitude < -180 || c.EndPoint.Longitude > 180 {
		return "", false, &domainerrors.ValidationError{
			Field:   "end_point.longitude",
			Message: "must be between -180 and 180",
		}
	}
	return "", false, nil
}

func CheckPrice(c *CreateTripCommand) (string, bool, error) {
	if c.Price < 0 {
		return "", false, &domainerrors.ValidationError{
			Field:   "price",
			Message: "must be greater than or equal to 0",
		}
	}
	return "valid", true, nil
}

func Chain(checks ...Check) Check {
	return func(c *CreateTripCommand) (string, bool, error) {
		for _, check := range checks {
			decision, handled, err := check(c)
			if err != nil {
				return "", false, err
			}
			if handled {
				return decision, true, nil
			}
		}
		return "", false, domainerrors.ErrValidationChainBroken
	}
}

func (c *CreateTripCommand) Validate() error {
	check := Chain(CheckUserID, CheckDriverID, CheckStartLatitude, CheckStartLongitude, CheckEndLatitude, CheckEndLongitude, CheckPrice)
	_, _, err := check(c)
	return err
}
