package daytime

import (
	"strings"
	"time"

	"github.com/benleb/automoli-go/internal/homeassistant"
	"github.com/benleb/automoli-go/internal/models/flash"
)

type Daytime struct {
	// Name is the name of the daytime
	Name string `json:"name" mapstructure:"name"`

	// MotionDetection is a flag to enable/disable motion detection for this daytime
	// motionDetection bool `mapstructure:"motion_detection"`

	// Start is the time when the daytime should be activated
	Start time.Time `json:"start" mapstructure:"start"`

	// LightConfiguration holds the light settings for the daytime
	LightConfiguration `mapstructure:",squash"`

	// Targets is the set of entity ids to apply the daytime/light configuration to (if not set, all room entities will be used)
	Targets targets `json:"target,omitempty" mapstructure:"target,omitempty"`

	// BrightnessPct is the brightness percentage to set for the target entities
	BrightnessPct *uint8 `json:"brightness,omitempty" mapstructure:"brightness,omitempty"`

	// ServiceData contains additional options that will be used to activate the daytime
	// These settings will be sent to home assistant as "service data".
	// check the home assistant "light.turn_on" service docs for available options
	// → https://www.home-assistant.io/integrations/light#service-lightturn_on
	ServiceData map[string]interface{} `json:"service_data,omitempty" mapstructure:"service_data,omitempty"`
}

// ManualModeConfiguration holds settings for the manual mode (lights turned on manually).
type ManualModeConfiguration struct {
	// LockConfiguration is a flag to lock the light configuration if the light was manually turned on
	LockConfiguration bool `json:"lock_configuration,omitempty" mapstructure:"lock_configuration,omitempty"`

	// LockState is a flag to prevent the lights from being turned off automatically if they were manually turned on
	LockState bool `json:"lock_state,omitempty" mapstructure:"lock_state,omitempty"`
}

// LightConfiguration holds settings controlling the light behavior.
type LightConfiguration struct {
	//  Delay is the time after which the lights should be turned off if no motion is detected.
	Delay time.Duration `json:"delay,omitempty" mapstructure:"delay,omitempty"`

	// Transition is the transition time in seconds to slowly turn on/off the lights
	Transition time.Duration `json:"transition,omitempty" mapstructure:"transition,omitempty"`

	// Flash flashes the lights. Available options: short & long
	Flash flash.Flash `json:"flash,omitempty" mapstructure:"flash,omitempty"`

	// ManualModeConfiguration holds settings for the manual mode (lights turned on manually)
	ManualModeConfiguration `json:"manual,omitempty" mapstructure:"manual,omitempty"`
}

// targets is a set of home assistant entity IDs.
type targets []homeassistant.EntityID

// UnmarshalText implements the encoding.TextUnmarshaler interface
// (used by mapstructure to map string/slice from the config file to a slice).
func (t *targets) UnmarshalText(text []byte) error {
	for _, rawEntityID := range strings.Split(string(text), ";") {
		if entityID, err := homeassistant.NewEntityID(rawEntityID); err == nil {
			*t = append(*t, *entityID)
		}
	}

	return nil
}
