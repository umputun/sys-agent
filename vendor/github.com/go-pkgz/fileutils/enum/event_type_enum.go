// Code generated by enum generator; DO NOT EDIT.
package enum

import (
	"fmt"

	"database/sql/driver"
	"strings"
)

// EventType is the exported type for the enum
type EventType struct {
	name  string
	value int
}

func (e EventType) String() string { return e.name }

// MarshalText implements encoding.TextMarshaler
func (e EventType) MarshalText() ([]byte, error) {
	return []byte(e.name), nil
}

// UnmarshalText implements encoding.TextUnmarshaler
func (e *EventType) UnmarshalText(text []byte) error {
	var err error
	*e, err = ParseEventType(string(text))
	return err
}

// Value implements the driver.Valuer interface
func (e EventType) Value() (driver.Value, error) {
	return e.name, nil
}

// Scan implements the sql.Scanner interface
func (e *EventType) Scan(value interface{}) error {
	if value == nil {
		*e = EventTypeValues()[0]
		return nil
	}

	str, ok := value.(string)
	if !ok {
		if b, ok := value.([]byte); ok {
			str = string(b)
		} else {
			return fmt.Errorf("invalid eventType value: %v", value)
		}
	}

	val, err := ParseEventType(str)
	if err != nil {
		return err
	}

	*e = val
	return nil
}

// ParseEventType converts string to eventType enum value
func ParseEventType(v string) (EventType, error) {

	switch strings.ToLower(v) {
	case strings.ToLower("Chmod"):
		return EventTypeChmod, nil
	case strings.ToLower("Create"):
		return EventTypeCreate, nil
	case strings.ToLower("Remove"):
		return EventTypeRemove, nil
	case strings.ToLower("Rename"):
		return EventTypeRename, nil
	case strings.ToLower("Write"):
		return EventTypeWrite, nil

	}

	return EventType{}, fmt.Errorf("invalid eventType: %s", v)
}

// MustEventType is like ParseEventType but panics if string is invalid
func MustEventType(v string) EventType {
	r, err := ParseEventType(v)
	if err != nil {
		panic(err)
	}
	return r
}

// Public constants for eventType values
var (
	EventTypeChmod  = EventType{name: "Chmod", value: 4}
	EventTypeCreate = EventType{name: "Create", value: 0}
	EventTypeRemove = EventType{name: "Remove", value: 2}
	EventTypeRename = EventType{name: "Rename", value: 3}
	EventTypeWrite  = EventType{name: "Write", value: 1}
)

// EventTypeValues returns all possible enum values
func EventTypeValues() []EventType {
	return []EventType{
		EventTypeChmod,
		EventTypeCreate,
		EventTypeRemove,
		EventTypeRename,
		EventTypeWrite,
	}
}

// EventTypeNames returns all possible enum names
func EventTypeNames() []string {
	return []string{
		"Chmod",
		"Create",
		"Remove",
		"Rename",
		"Write",
	}
}
