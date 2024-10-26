package automoli

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/benleb/automoli-go/internal/homeassistant"
	"github.com/benleb/automoli-go/internal/icons"
	"github.com/benleb/automoli-go/internal/models"
	"github.com/benleb/automoli-go/internal/models/daytime"
	"github.com/benleb/automoli-go/internal/models/flash"
	"github.com/benleb/automoli-go/internal/style"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/go-co-op/gocron"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

var (
	AppName    = "AutoMoLi"
	AppIcon    = icons.LightOn
	AppVersion = "dev"
	CommitDate = "unknown"
	Commit     = "none"
)

type AutoMoLi struct {
	// Config holds the global configuration for AutoMoLi.
	*Config `mapstructure:",squash"`

	// Pr is the global (pretty) printer for AutoMoLi.
	Pr *log.Logger

	// rooms holds all rooms that are managed by AutoMoLi.
	rooms []*Room

	// ha is the Home Assistant client.
	ha *homeassistant.HomeAssistant

	// channel for incoming events from Home Assistant
	events chan *homeassistant.EventMsg

	// a sensor -> room mapping to forward incoming events to the correct room.
	roomSensorEvents map[homeassistant.EntityID]map[homeassistant.EventType]*Room

	triggerEvents mapset.Set[homeassistant.EventType]

	// daytime switcher
	daytimeSwitcher *gocron.Scheduler

	// room style
	style lipgloss.Style

	// counter
	eventsReceivedTotal atomic.Uint64

	// time when AutoMoLi was started
	startTime time.Time

	lastEventReceived time.Time
}

func New() *AutoMoLi {
	coloredAppName := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0099")).SetString(AppName)

	aml := &AutoMoLi{
		Config: &Config{
			StatsInterval: viper.GetDuration("automoli.defaults.stats_interval"),

			LightConfiguration: daytime.LightConfiguration{
				Transition: viper.GetDuration("automoli.defaults.transition"),
				Flash:      flash.Flash(viper.GetString("automoli.defaults.flash")),
				Delay:      viper.GetDuration("automoli.defaults.delay"),
			},
		},

		events:           make(chan *homeassistant.EventMsg),
		roomSensorEvents: make(map[homeassistant.EntityID]map[homeassistant.EventType]*Room),
		triggerEvents:    mapset.NewSet[homeassistant.EventType](),

		daytimeSwitcher: gocron.NewScheduler(time.UTC),

		style: lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0099")),
		Pr:    models.Printer.WithPrefix(coloredAppName.Faint(true).Render()),

		startTime: time.Now(),
	}

	// unmarshal global configuration
	if err := viper.UnmarshalKey("automoli", &aml.Config, viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(mapstructure.StringToTimeDurationHookFunc(), homeassistant.StringToEntityIDHookFunc()))); err != nil {
		aml.Pr.With("err", err).Error("decoding default room config failed")

		return nil
	}

	// create homeassistant client
	url := viper.GetString("homeassistant.url")
	token := viper.GetString("homeassistant.token")

	// create homeassistant client
	aml.ha = aml.createHomeAssistantSession(url, token)

	aml.Pr.Infof("%s Home Assistant session created", icons.GreenTick)

	//
	// rooms configuration

	// check if rooms are configured and valid
	roomConfig, ok := viper.Get("rooms").([]interface{})
	if !ok || len(roomConfig) == 0 {
		aml.Pr.Error("room config not found")

		return nil
	}

	rooms := parseRooms(aml, roomConfig)
	if len(rooms) == 0 {
		aml.Pr.Errorf("no valid rooms found - room config: %+v", roomConfig...)

		return nil
	}

	aml.rooms = rooms

	// collect all trigger events &
	for _, room := range aml.rooms {
		// subscribe to xiaomi motion events
		room.TriggerEvents.Add(homeassistant.EventXiaomiMotion)

		// subscribe state_changed
		if room.MotionStateOn != "" && room.MotionStateOff != "" {
			room.TriggerEvents.Add(homeassistant.EventStateChanged)
		}

		aml.triggerEvents = aml.triggerEvents.Union(room.TriggerEvents)

		// create a sensor -> room mapping to forward incoming events to the correct room
		for _, sensor := range room.MotionSensors {
			for _, eventType := range room.TriggerEvents.ToSlice() {
				if _, ok := aml.roomSensorEvents[sensor]; !ok {
					aml.roomSensorEvents[sensor] = make(map[homeassistant.EventType]*Room)
				}

				aml.roomSensorEvents[sensor][eventType] = room
			}
		}

		// print room config
		fmt.Println(room.GetFmtRoomConfig())
	}

	// get all lights from all rooms
	allLights := mapset.NewSet[homeassistant.EntityID]()
	for _, room := range aml.rooms {
		allLights = allLights.Union(mapset.NewSet[homeassistant.EntityID](room.Lights...))
	}

	// print loaded rooms, lights & sensors
	intro := strings.Builder{}
	intro.WriteString(coloredAppName.Render())
	intro.WriteString(" " + style.DarkDivider.String() + " ")

	// magic hex code
	intro.WriteString(" " + icons.Home + " ")
	intro.WriteString(style.Bold(aml.hashedHouseID(len(aml.rooms), allLights.Cardinality(), len(aml.roomSensorEvents))) + " ")
	// rooms
	intro.WriteString(" " + style.DarkDivider.String() + " ")
	intro.WriteString(" " + icons.Door + " ")
	intro.WriteString(style.Bold(strconv.Itoa(len(aml.rooms))))
	intro.WriteString(style.Gray(8).Render(" rooms "))
	// lights
	intro.WriteString(" " + style.DarkDivider.String() + " ")
	intro.WriteString(" " + icons.LightOn + " ")
	intro.WriteString(style.Bold(strconv.Itoa(allLights.Cardinality())))
	intro.WriteString(style.Gray(8).Render(" lights "))
	// sensors
	intro.WriteString(" " + style.DarkDivider.String() + " ")
	intro.WriteString(" " + icons.Motion + " ")
	intro.WriteString(style.Bold(strconv.Itoa(len(aml.roomSensorEvents))))
	intro.WriteString(style.Gray(8).Render(" sensors "))

	fmt.Println(lipgloss.NewStyle().Padding(1, 0).Render(intro.String()))

	// start daytime switcher
	aml.daytimeSwitcher.StartAsync()

	// start event handler
	go aml.eventHandler()

	// start stats ticker
	go aml.statsTicker()

	// subscribe to events
	aml.ha.SubscribeToEvents(aml.triggerEvents)

	return aml
}

// hashedHouseID creates a magic house id based on the number of rooms, lights and sensors.
// The ID is a single, short, unique but also stable identifier for the current configuration of rooms, lights and sensors.
func (aml *AutoMoLi) hashedHouseID(roomCount, lightCount, sensorCount int) string {
	hashee := fmt.Sprintf("rooms: %d - lights: %d - sensors: %d", roomCount, lightCount, sensorCount)

	// create a hash / (full) house id from the hashee string
	fullHouseID := sha256.New()
	fullHouseID.Write([]byte(hashee))
	houseID := fullHouseID.Sum(nil)

	log.Debugf("house id: %X", houseID)

	// create a 3 char hex code from the hash
	return fmt.Sprintf("%X", houseID)[:3]
}

func (aml *AutoMoLi) createHomeAssistantSession(url, token string) *homeassistant.HomeAssistant {
	// create homeassistant session
	hass, err := homeassistant.New(url, token, &aml.events)
	if err != nil {
		aml.Pr.Error(err)

		os.Exit(1)
	}

	aml.lastEventReceived = time.Now()

	return hass
}

// statsTicker prints the stats about sent/received messages in a regular interval.
func (aml *AutoMoLi) statsTicker() {
	aml.Pr.Info(icons.Stopwatch + " event counter started")

	ticker := time.NewTicker(viper.GetDuration("automoli.defaults.stats_interval"))

	fmtUnit := style.LightGray.Render("/m")
	perSecondFormat := "%3.1f"

	fmtStats := func(eventsTotal uint64, eventsPerTime float64, roomStyle lipgloss.Style) string {
		return fmt.Sprintf("%d%s%s", eventsTotal, roomStyle.Bold(true).Render("|"), fmt.Sprintf(perSecondFormat, eventsPerTime)+fmtUnit)
	}

	for range ticker.C {
		totalEvents := aml.eventsReceivedTotal.Load()
		totalEventsPerTime := float64(totalEvents) / time.Since(aml.startTime).Minutes()

		fmtEventCounts := []string{fmtStats(totalEvents, totalEventsPerTime, aml.style)}

		for _, room := range aml.rooms {
			eventsReceived := room.eventsReceivedTotal.Load()
			eventsPerTime := float64(eventsReceived) / time.Since(aml.startTime).Minutes()

			fmtRoomEventCount := strings.Builder{}

			// show an icon if the lights are on
			if room.isLightOn() {
				fmtRoomEventCount.WriteString(icons.LightOn + " ")
			}

			fmtRoomEventCount.WriteString(room.FmtShort())
			fmtRoomEventCount.WriteString(style.Gray(6).Render(":"))
			fmtRoomEventCount.WriteString(fmtStats(eventsReceived, eventsPerTime, room.style))

			fmtEventCounts = append(fmtEventCounts, fmtRoomEventCount.String())
		}

		fmt.Println()
		aml.Pr.Print(strings.Join(fmtEventCounts, " | "))
		fmt.Println()
	}
}

// eventHandler listens for incoming events from Home Assistant and forwards them to the corresponding room.
func (aml *AutoMoLi) eventHandler() {
	aml.Pr.Infof("event handler started | channel: %+v", aml.events)

	for triggerEvent := range aml.events {
		// count events
		aml.eventsReceivedTotal.Add(1)

		entityID := triggerEvent.Event.Data.EntityID

		// get the room this event belongs to
		if room, ok := aml.roomSensorEvents[entityID][triggerEvent.Event.Type]; ok {
			room.EventsChannel <- triggerEvent
		}

		aml.lastEventReceived = time.Now()

		aml.Pr.Debugf("%s no room found for sensor %v", icons.Hae, entityID)
	}
}

// isDisabled checks if AutoMoLi is disabled by any entity or entity state.
func (aml *AutoMoLi) isDisabled() bool {
	return len(aml.disabledBy()) > 0
}

func (aml *AutoMoLi) disabledBy() map[homeassistant.EntityID]string {
	activeDisabler := make(map[homeassistant.EntityID]string)

	for disablingEntityID, disablingStates := range aml.DisabledBy {
		if entityState := aml.ha.GetState(disablingEntityID).State; mapset.NewSet[string](disablingStates...).Contains(entityState) {
			activeDisabler[disablingEntityID] = entityState
		}
	}

	return activeDisabler
}
