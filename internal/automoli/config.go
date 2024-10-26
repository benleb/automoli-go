package automoli

import (
	"math"
	"time"

	"github.com/benleb/automoli-go/internal/homeassistant"
	"github.com/benleb/automoli-go/internal/icons"
	"github.com/benleb/automoli-go/internal/models"
	"github.com/benleb/automoli-go/internal/models/daytime"
	"github.com/benleb/automoli-go/internal/style"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/mitchellh/mapstructure"
)

type Config struct {
	// DisabledBy is a map of entities that control the state of AutoMoLi
	// if any entity is in state 'off' - AutoMoLi won't react to any events
	DisabledBy map[homeassistant.EntityID][]string `mapstructure:"disabled_by,omitempty"`

	// StatsInterval is the interval in which the stats ticker will print the stats line
	StatsInterval time.Duration `mapstructure:"stats_interval,omitempty"`

	daytime.LightConfiguration `mapstructure:",squash"`
}

func parseRooms(aml *AutoMoLi, roomConfig []interface{}) []*Room {
	rooms := make([]*Room, 0)

	for _, rawRoom := range roomConfig {
		rawRoom, ok := rawRoom.(map[string]interface{})
		if !ok {
			log.Errorf("❌ invalid room config: %+v", rawRoom)

			continue
		}

		// create a room
		if room := newRoom(aml, rawRoom); room != nil {
			room.aml = aml

			// start event receiver
			go room.eventReceiver()

			// schedule daytime switches
			go room.scheduleDaytimeSwitches()

			// initial setup depending on current light state
			if room.isLightOn() {
				room.pr.Infof("%s lights on! starting the timer...", icons.LightOn)

				room.refreshTimer()
			}

			rooms = append(rooms, room)
		}
	}

	return rooms
}

func newRoom(aml *AutoMoLi, rawRoom map[string]interface{}) *Room {
	// room with default settings
	room := &Room{
		ha: aml.ha,

		LightConfiguration: daytime.LightConfiguration{
			Delay:      aml.Delay,
			Transition: aml.Transition,
			Flash:      aml.Flash,
		},

		TriggerEvents: mapset.NewSet[homeassistant.EventType](),

		EventsChannel: make(chan *homeassistant.EventMsg, 16),
	}

	// create rooms decoder
	var metadata mapstructure.Metadata

	decoder, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeHookFunc("15:04"),
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.TextUnmarshallerHookFunc(),
			homeassistant.StringToEntityIDHookFunc(),
		),
		Result:   &room,
		Metadata: &metadata,
	})

	// decode room config
	err := decoder.Decode(rawRoom)
	if err != nil {
		log.With("err", err).Error("❌ decoding room config failed")

		return nil
	} else if len(metadata.Unused) > 0 {
		aml.Pr.With("unused", metadata.Unused).Infof("❔ %s has nused config entries", style.Bold(room.Name))
	}

	//
	// pretty print 💄

	// create room color & style
	room.color = GenerateColorFromString(room.Name)
	room.style = lipgloss.NewStyle().Foreground(room.color)

	// create room logger/printer
	room.pr = aml.Pr.WithPrefix(room.style.Render(room.Name))

	//
	// validity check

	// check if light & sensors are configured
	switch {
	case len(room.Lights) == 0:
		room.pr.Errorf("❌ no lights configured for %+v | disabling %s for this room", style.Bold(room.Name), models.AppName)

		return nil

	case len(room.MotionSensors) == 0:
		room.pr.Errorf("❌ no motion sensors configured for %+v | disabling %s for this room", style.Bold(room.Name), models.AppName)

		return nil

	case room.findActiveDaytime() < 0:
		room.pr.Errorf("❌ no active daytime found for %+v | disabling %s for this room", style.Bold(room.Name), models.AppName)

		return nil
	}

	//
	// daytimes

	// settings
	for _, currentDaytime := range room.Daytimes {
		// set targets to room lights if not explicitly set
		if len(currentDaytime.Targets) == 0 {
			currentDaytime.Targets = room.Lights
		}

		// set daytime off-delay
		if currentDaytime.Delay == 0 {
			currentDaytime.Delay = room.Delay
		}

		// if a custom service data is set, we use it
		serviceData := make(map[string]interface{})

		if len(currentDaytime.ServiceData) > 0 {
			serviceData = currentDaytime.ServiceData
		}

		// set daytime transition times
		if currentDaytime.Transition == 0 {
			currentDaytime.Transition = room.Transition
		}

		// service_data takes precedence over transition time wrapper field
		if _, ok := serviceData["transition"]; !ok {
			serviceData["transition"] = currentDaytime.Transition.Seconds()
		}

		// set brightness_pct
		if currentDaytime.BrightnessPct != nil && *currentDaytime.BrightnessPct > 0 {
			// restrict brightness to 0-100
			brightnessPct := uint8(math.Min(math.Max(float64(*currentDaytime.BrightnessPct), 0), 100))

			// service_data takes precedence over brightness wrapper field
			if _, ok := serviceData["brightness_pct"]; !ok {
				serviceData["brightness_pct"] = brightnessPct
			}
		}

		currentDaytime.ServiceData = serviceData
	}

	return room
}
