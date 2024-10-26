package homeassistant

import (
	"time"
)

var (
	EventStateChanged         = EventType("state_changed")
	EventXiaomiMotion         = EventType("xiaomi_aqara.motion")
	EventHomeAssistantStart   = EventType("homeassistant_start")
	EventHomeAssistantStarted = EventType("homeassistant_started")
)

type EventType string

type event struct {
	Type      EventType    `json:"event_type" mapstructure:"event_type"`
	Origin    string       `json:"origin"     mapstructure:"origin"`
	TimeFired time.Time    `json:"time_fired" mapstructure:"time_fired"`
	Context   stateContext `json:"context"    mapstructure:"context"`
	Data      eventData    `json:"data"       mapstructure:"data"`
}

type eventData struct {
	EntityID EntityID `json:"entity_id" mapstructure:"entity_id"`
	NewState State    `json:"new_state" mapstructure:"new_state"`
	OldState State    `json:"old_state" mapstructure:"old_state"`
}

type State struct {
	EntityID    EntityID     `json:"entity_id"    mapstructure:"entity_id"`
	State       string       `json:"state"        mapstructure:"state"`
	LastChanged time.Time    `json:"last_changed" mapstructure:"last_changed"`
	LastUpdated time.Time    `json:"last_updated" mapstructure:"last_updated"`
	Context     stateContext `json:"context"      mapstructure:"context"`
	Attributes  attributes   `json:"attributes"   mapstructure:"attributes"`
}

type stateContext struct {
	ID       string `json:"id"        mapstructure:"id"`
	ParentID string `json:"parent_id" mapstructure:"parent_id"`
	UserID   string `json:"user_id"   mapstructure:"user_id"`
}

type attributes struct {
	FriendlyName      string                 `json:"friendly_name"       mapstructure:"friendly_name"`
	Icon              string                 `json:"icon"                mapstructure:"icon"`
	DeviceClass       string                 `json:"device_class"        mapstructure:"device_class"`
	StateClass        string                 `json:"state_class"         mapstructure:"state_class"`
	UnitOfMeasurement string                 `json:"unit_of_measurement" mapstructure:"unit_of_measurement"`
	SupportedFeatures int64                  `json:"supported_features"  mapstructure:"supported_features"`
	Other             map[string]interface{} `mapstructure:",remain"`
}
