package automoli

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/benleb/automoli-go/internal/homeassistant"
	"github.com/benleb/automoli-go/internal/icons"
	"github.com/benleb/automoli-go/internal/models"
	"github.com/benleb/automoli-go/internal/models/daytime"
	"github.com/benleb/automoli-go/internal/models/domain"
	"github.com/benleb/automoli-go/internal/models/flash"
	"github.com/benleb/automoli-go/internal/models/service"
	"github.com/benleb/automoli-go/internal/style"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/kr/pretty"
	"github.com/spf13/viper"
	"golang.org/x/exp/slices"
)

type Room struct {
	aml *AutoMoLi
	ha  *homeassistant.HomeAssistant `mapstructure:"-"`

	Name string `json:"name" mapstructure:"name"`

	daytime.LightConfiguration `mapstructure:",squash"`

	// //  Delay is the time after which the lights should be turned off if no motion is detected.
	// Delay *time.Duration `json:"delay,omitempty" mapstructure:"delay,omitempty"`

	// // Transition is the transition time in seconds to slowly turn on/off the lights
	// Transition *time.Duration `json:"transition,omitempty" mapstructure:"transition,omitempty"`
	// Flash      *time.Duration `json:"flash,omitempty"      mapstructure:"flash,omitempty"`

	// // IgnoreDumbLightsForStateCheck ignores dumb lights (supportedFeatures: 0, e.g. switches, ...) for the state check.
	// IgnoreDumbLightsForStateCheck bool `json:"ignore_dumb_lights_for_state_check,omitempty" mapstructure:"ignore_dumb_lights_for_state_check,omitempty"`
	// // DoubleSwitchDumbLights switches on/off dumb lights (supportedFeatures: 0, e.g. switches, ...) twice to turn them on/off.
	// DoubleSwitchDumbLights bool `json:"double_switch_dumb_lights,omitempty" mapstructure:"double_switch_dumb_lights,omitempty"`

	Lights []homeassistant.EntityID `json:"lights" mapstructure:"lights"`

	MotionSensors  []homeassistant.EntityID `json:"motion_sensors"             mapstructure:"motion_sensors"`
	MotionStateOn  string                   `json:"motion_state_on,omitempty"  mapstructure:"motion_state_on,omitempty"`
	MotionStateOff string                   `json:"motion_state_off,omitempty" mapstructure:"motion_state_off,omitempty"`

	// sensors & threshold for humidity check
	HumiditySensors   []homeassistant.EntityID `json:"humidity_sensors,omitempty"   mapstructure:"humidity_sensors,omitempty"`
	HumidityThreshold *uint8                   `json:"humidity_threshold,omitempty" mapstructure:"humidity_threshold,omitempty"`

	// daytimes
	Daytimes           []*daytime.Daytime `json:"daytimes" mapstructure:"daytimes"`
	activeDaytimeIndex int

	EventsChannel chan *homeassistant.EventMsg

	TriggerEvents mapset.Set[homeassistant.EventType]

	turnOffTimer *time.Timer

	color lipgloss.Color
	style lipgloss.Style
	pr    *log.Logger

	lastSwitchedOn  time.Time
	lastSwitchedOff time.Time

	// mutex to prevent concurrent access to the room
	sync.Mutex

	// counter
	eventsReceivedTotal atomic.Uint64

	// TODO
	// Alias []string `json:"alias" mapstructure:"alias,omitempty"`
	// DisableHueGroups     bool `json:"disable_hue_groups" mapstructure:"disable_hue_groups"`
	// ThresholdIlluminance int  `json:"illuminance_threshold,omitempty" mapstructure:"illuminance_threshold,omitempty"`
	// ThresholdHumidity    int  `json:"humidity_threshold,omitempty" mapstructure:"humidity_threshold,omitempty"`
	// Dim       DimSettings       `json:"dim,omitempty" mapstructure:"dim,omitempty"`
	// NightMode NightModeSettings `json:"night_mode,omitempty" mapstructure:"night_mode,omitempty"`
	// transition to new daytime on daytime switch
	// transitionOnDaytimeSwitch bool `mapstructure:"transition_on_daytime_switch,omitempty"`
}

func (r *Room) String() string {
	return r.Name
}

func (r *Room) FmtString() string {
	return r.style.Render(r.Name)
}

func (r *Room) FmtShort() string {
	return r.style.Render(strings.ReplaceAll(r.Name, "room", ""))
}

func (r *Room) fmtDisabler() []string {
	// format disabling entities & states
	disabler := make([]string, 0, len(r.aml.disabledBy()))
	for disablingEntityID, disablingState := range r.aml.disabledBy() {
		disabler = append(disabler, disablingEntityID.FmtString()+style.Gray(5).Render("=")+style.Bold(disablingState))
	}

	return disabler
}

func (r *Room) GetActiveDaytime() *daytime.Daytime {
	return r.Daytimes[r.activeDaytimeIndex]
}

func (r *Room) GetActiveDelay() time.Duration {
	return r.GetActiveDaytime().Delay
}

func (r *Room) findActiveDaytime() int {
	now := time.Now()

	for _, dt := range r.Daytimes {
		// we set the proper date (today) for the daytime start time as we only
		// get the time itself from the config. this is necessary to compare
		// daytime start times with the current time to conviniently check which
		// daytime is active
		dt.Start = time.Date(now.Year(), now.Month(), now.Day(), dt.Start.Hour(), dt.Start.Minute(), 0, 0, now.Location())
	}

	// sort daytimes by start time
	sort.Slice(r.Daytimes, func(i, j int) bool {
		return r.Daytimes[i].Start.Before(r.Daytimes[j].Start)
	})

	for idx, currentDaytime := range r.Daytimes {
		// get next/following daytime
		nextIdx := (idx + 1) % len(r.Daytimes)
		nextDaytime := r.Daytimes[nextIdx]

		// this daytime start is before now
		startBeforeNow := currentDaytime.Start.Before(now)
		// next daytime start is after now
		nextStartAfterNow := nextDaytime.Start.After(now)

		if startBeforeNow && nextStartAfterNow {
			// set daytime as active if both conditions are true
			r.activeDaytimeIndex = slices.Index(r.Daytimes, currentDaytime)

			break
		}

		// set last daytime as active if no other daytime is
		if idx == len(r.Daytimes)-1 && r.activeDaytimeIndex == 0 {
			r.activeDaytimeIndex = slices.Index(r.Daytimes, currentDaytime)
		}
	}

	return r.activeDaytimeIndex
}

func (r *Room) refreshTimer() {
	delay := r.GetActiveDelay()

	if r.turnOffTimer == nil {
		r.turnOffTimer = time.NewTimer(delay)

		// starting off switcher for this room
		go r.offSwitcher()
	} else {
		r.turnOffTimer.Reset(delay)
	}

	r.pr.Debugf("⏰ timer reset | turning off the lights in %s", delay)
}

// currentMaxHumidity finds the highest humidity value of all humidity sensors in the room.
func (r *Room) currentMaxHumidity() (homeassistant.EntityID, uint8) {
	var currentMaxHumiditySensor homeassistant.EntityID

	currentMax := 0.0

	for _, sensor := range r.HumiditySensors {
		currentHumidity, err := strconv.ParseFloat(r.ha.GetState(sensor).State, 64)
		if err != nil {
			r.pr.Errorf("%s invalid humidity value '%+v' from entity: %s", icons.Splash, r.ha.GetState(sensor).State, sensor.FmtString())

			continue
		}

		if currentHumidity > currentMax {
			currentMax = currentHumidity
			currentMaxHumiditySensor = sensor
		}
	}

	r.pr.Infof("current max humidity: %+v | sensor: %+v", currentMax, currentMaxHumiditySensor.FmtString())

	return currentMaxHumiditySensor, uint8(currentMax)
}

// IsHumidityAboveThreshold checks if any humidity sensor in the room is above the threshold.
func (r *Room) IsHumidityAboveThreshold() bool {
	// if no humidity sensors are configured, we won't check the humidity
	if r.HumidityThreshold == nil {
		return false
	}

	// check if the current max humidity is above the threshold
	if currentMaxHumiditySensor, currentMaxHumidity := r.currentMaxHumidity(); currentMaxHumiditySensor != (homeassistant.EntityID{}) && currentMaxHumidity > *r.HumidityThreshold {
		// fmt.Printf("humidity above threshold 💦 current max humidity: %+v | sensor: %+v\n", currentMaxHumidity, currentMaxHumiditySensor)
		return true
	}

	return false
}

// isLightOn checks if any as light configured entity in the room is on.
func (r *Room) isLightOn() bool {
	return len(r.lightsOn()) > 0
}

// lightsOn gets returns all lights that are currently on.
func (r *Room) lightsOn() []homeassistant.EntityID {
	onLights := make([]homeassistant.EntityID, 0)

	for _, light := range r.Lights {
		if entityState := r.ha.GetState(light); entityState != nil && entityState.State == "on" {
			onLights = append(onLights, light)
		}
	}

	r.pr.Debugf("lights on: %+v", onLights)

	return onLights
}

func (r *Room) isDisabledByLightConfiguration() bool {
	return r.disabledByLightConfiguration(r.GetActiveDaytime())
}

func (r *Room) disabledByLightConfiguration(activeDaytime *daytime.Daytime) bool {
	// brightness is set
	if activeDaytime.BrightnessPct != nil && *activeDaytime.BrightnessPct == 0 {
		return true
	}

	// target/scene is set
	if targets := activeDaytime.Targets; len(targets) > 0 {
		return false
	}

	// brightness is set
	if activeDaytime.BrightnessPct != nil && *activeDaytime.BrightnessPct > 0 {
		return false
	}

	// custom service data is set
	if len(activeDaytime.ServiceData) > 0 {
		return false
	}

	return true
}

func (r *Room) turnLightsOn(triggerEvent *homeassistant.EventMsg) bool {
	// get the active daytime/light configuration
	activeDaytime := r.GetActiveDaytime()

	// record
	eventToCallDuration := time.Since(triggerEvent.Event.TimeFired)

	// turn on the lights & set state
	turnOnResults := r.ha.TurnOn(activeDaytime.Targets, activeDaytime.ServiceData)

	// record
	eventToLightDuration := time.Since(triggerEvent.Event.TimeFired)

	// construct turned on message
	turnedOnMsg := strings.Builder{}
	turnedOnMsg.WriteString(icons.LightOn)
	turnedOnMsg.WriteString(" turned " + style.Bold("on") + " ")
	turnedOnMsg.WriteString("→ " + r.FormatDaytimeConfiguration(activeDaytime) + " ")
	turnedOnMsg.WriteString(style.DarkDivider.String() + " ")
	turnedOnMsg.WriteString(style.LightGray.Render("delay") + r.style.Render(": "))
	turnedOnMsg.WriteString(fmt.Sprint(r.GetActiveDelay()))
	// add event-to-call and event-to-light duration
	turnedOnMsg.WriteString(" " + style.DarkDivider.String() + " ")
	turnedOnMsg.WriteString(style.LightGray.Render("etc") + r.style.Render(":"))
	turnedOnMsg.WriteString(eventToCallDuration.Truncate(time.Millisecond).String())
	turnedOnMsg.WriteString(style.DarkerDivider.String())
	turnedOnMsg.WriteString(style.LightGray.Render("etl") + r.style.Render(":"))
	turnedOnMsg.WriteString(eventToLightDuration.Truncate(time.Millisecond).String())

	r.pr.Print(turnedOnMsg.String())

	for _, result := range turnOnResults.ToSlice() {
		if result.Success {
			r.lastSwitchedOn = time.Now()

			return true
		}
	}

	return false
}

func (r *Room) turnLightsOff(timeFired time.Time) {
	r.pr.Debugf("%s %s: validating conditions...", icons.Checklist, service.TurnOff.FmtString())

	activeDaytime := r.GetActiveDaytime()

	// add service data (for turn_off only flash and transition are supported)
	serviceData := make(map[string]interface{})

	if activeDaytime.Transition >= 0 {
		serviceData["transition"] = activeDaytime.Transition.Seconds()
	}

	if activeDaytime.Flash == flash.Short || activeDaytime.Flash == flash.Long {
		serviceData["flash"] = activeDaytime.Flash
	}

	// record
	eventToCallDuration := time.Since(timeFired)

	// turn off the lights
	_ = r.ha.TurnOff(r.Lights, serviceData)

	// record
	eventToLightDuration := time.Since(timeFired)

	r.lastSwitchedOff = time.Now()

	// log
	turnedOffMsg := strings.Builder{}
	turnedOffMsg.WriteString(icons.LightOff + " ")
	turnedOffMsg.WriteString("no motion for ")
	turnedOffMsg.WriteString(style.Bold(activeDaytime.Delay.String()))
	turnedOffMsg.WriteString(" " + r.style.Faint(true).Render("→") + " ")
	turnedOffMsg.WriteString("turned" + style.Bold(" off"))

	if lightOnDuration := r.lastSwitchedOff.Sub(r.lastSwitchedOn); lightOnDuration > 0 && r.lastSwitchedOn != (time.Time{}) {
		turnedOffMsg.WriteString(" " + style.DarkDivider.String() + " ")
		turnedOffMsg.WriteString(style.LightGray.Render("after ") + lightOnDuration.Round(time.Second).String())
	}

	turnedOffMsg.WriteString(" " + style.DarkDivider.String() + " ")
	turnedOffMsg.WriteString(style.LightGray.Render("etc") + r.style.Render(":"))
	turnedOffMsg.WriteString(eventToCallDuration.Truncate(time.Millisecond).String())
	turnedOffMsg.WriteString(style.DarkerDivider.String())
	turnedOffMsg.WriteString(style.LightGray.Render("etl") + r.style.Render(":"))
	turnedOffMsg.WriteString(eventToLightDuration.Truncate(time.Millisecond).String())

	r.pr.Print(turnedOffMsg.String())
}

func (r *Room) offSwitcher() {
	r.pr.Debugf("%s starting off-switcher", icons.LightOff)

	for {
		timeFired := <-r.turnOffTimer.C

		// check if the lights are still on (could have been turned off manually or so)
		if !r.isLightOn() {
			r.pr.Info(style.LightGray.Render(icons.LightOff+" lights already") + " off")

			continue

		case r.aml.isDisabled():
			// 🚫 the disabled case 🚫
			// print disabling entities & states
			r.pr.Printf("%s %s prevented | disabled by: %+v", icons.Block, service.TurnOff.FmtStringStriketrough(), strings.Join(r.fmtDisabler(), " | "))

			continue

		case r.IsHumidityAboveThreshold():
			// 🚿 the shower case 🚿
			// check if someone might is taking a shower via humidity sensors
			// get the current max humidity sensor
			currentMaxHumiditySensor, currentMaxHumidity := r.currentMaxHumidity()

			notTurnedOffMsg := strings.Builder{}
			notTurnedOffMsg.WriteString(icons.Bath + " ")
			notTurnedOffMsg.WriteString(service.TurnOff.FmtStringStriketrough() + " ")
			notTurnedOffMsg.WriteString(style.DarkDivider.String() + " ")
			notTurnedOffMsg.WriteString(style.Bold("prevented ") + style.Gray(12).Render("by humidity sensor") + ": ")
			notTurnedOffMsg.WriteString(currentMaxHumiditySensor.FmtShort() + "(" + strconv.FormatUint(uint64(currentMaxHumidity), 10) + "%)\n")

			r.pr.Print(notTurnedOffMsg.String())

			continue
		}

		// turn off the lights
		r.turnLightsOff(timeFired)
	}
}

func (r *Room) FormatDaytimeConfiguration(daytime *daytime.Daytime) string {
	activeConfiguration := strings.Builder{}

	bright := style.Gray(8)
	dark := style.Gray(7)
	roomStyle := r.style.Faint(true)

	if daytime == r.GetActiveDaytime() {
		bright = style.BoldStyle
		dark = style.LightGray
		roomStyle = r.style
	}

	// service data
	serviceData := daytime.ServiceData

	switch {
	case r.disabledByLightConfiguration(daytime):
		daytime.Targets = make([]homeassistant.EntityID, 0)

		activeConfiguration.WriteString(bright.Faint(true).Italic(true).Render("none"))

	// case daytime.Targets != "" && daytime.Targets.Domain() == "scene":
	// 	activeConfiguration.WriteString(roomStyle.Render(daytime.Targets.Domain().String()) + style.Gray(6).Render(".") + bright.Render(daytime.Targets.EntityName()))
	case len(daytime.Targets) == 1 && daytime.Targets[0].Domain() == domain.Scene:
		activeConfiguration.WriteString(roomStyle.Render(daytime.Targets[0].Domain().String()) + style.Gray(6).Render(".") + bright.Render(daytime.Targets[0].EntityName()))

	case *daytime.BrightnessPct > 0:
		if len(daytime.Targets) > 0 {
			for _, target := range daytime.Targets[:1] {
				if target.Domain() == domain.Light {
					activeConfiguration.WriteString(roomStyle.Render(target.FmtShortWithStyles(r.style, bright))) // + style.Gray(6).Render(".") + bright.Render(daytime.Target.EntityName()))
				}
			}

			if len(daytime.Targets) > 1 {
				activeConfiguration.WriteString(roomStyle.Render(" +") + bright.Render(strconv.Itoa(len(daytime.Targets)-1)) + " ")
			}

			activeConfiguration.WriteString(" " + style.DarkIndicatorRight.String() + " ")
		}

		activeConfiguration.WriteString(bright.Render(strconv.FormatUint(uint64(*daytime.BrightnessPct), 10)) + roomStyle.Render("%"))

	case len(serviceData) > 0:
		opts := make([]string, 0)
		for opt, value := range serviceData {
			opts = append(opts, dark.Render(fmt.Sprintf("%s: %s", opt, bright.Render(fmt.Sprint(value)))))
		}

		activeConfiguration.WriteString(strings.Join(opts, " "))

	default:
		r.pr.Warnf("⚠️  invalid daytime configuration: %+v", daytime)
	}

	return activeConfiguration.String()
}

func (r *Room) GetFmtRoomConfig() string {
	out := strings.Builder{}

	//
	// daytimes
	daytimesList := make([]string, 0)

	for _, currentDaytime := range r.Daytimes {
		fmtDaytime := strings.Builder{}
		fmtDaytime.WriteString(currentDaytime.Start.Format("15:04"))
		// fmtDaytime.WriteString(style.DarkDivider.String())
		// fmtDaytime.WriteString(style.LightGray.Copy().Render(daytime.Delay.String()))
		fmtDaytime.WriteString(" " + currentDaytime.Name)
		fmtDaytime.WriteString(" " + r.style.Faint(true).Render("|"))
		fmtDaytime.WriteString(" " + r.FormatDaytimeConfiguration(currentDaytime))

		if currentDaytime == r.GetActiveDaytime() {
			daytimesList = append(daytimesList, listItemActive(fmtDaytime.String(), r.color))
		} else {
			daytimesList = append(daytimesList, listDaytimeItem(fmtDaytime.String()))
		}
	}

	//
	// lights
	lightsList := make([]string, 0)

	for _, light := range r.Lights {
		friendlyName := r.ha.FriendlyName(light)
		name := fmt.Sprintf("%s | %s", friendlyName, light.FmtShort())

		if entityState := r.ha.GetState(light); entityState != nil && entityState.State == "on" {
			lightsList = append(lightsList, listItemOn(name))
		} else {
			lightsList = append(lightsList, listItemStyle.UnsetWidth().Render(name))
		}
	}

	fmtLightsList := list.Render(
		lipgloss.JoinHorizontal(
			lipgloss.Top,
			listHeader(r.style.Align(lipgloss.Right).Faint(true).Render("lights")), //+GrayStyle.Render(":")),
			lipgloss.JoinVertical(lipgloss.Left, lightsList...),
		),
	)

	//
	// sensors
	sensorsList := make([]string, 0)

	for _, sensor := range r.MotionSensors {
		friendlyName := r.ha.FriendlyName(sensor)
		name := fmt.Sprintf("%s | %s", friendlyName, sensor.FmtShort())

		if entityState := r.ha.GetState(sensor); entityState != nil && entityState.State == "on" {
			sensorsList = append(sensorsList, listItemMotionOn(name))
		} else {
			sensorsList = append(sensorsList, listItem(name))
		}
	}

	fmtSensorsList := list.Render(
		lipgloss.JoinHorizontal(
			lipgloss.Top,
			listHeader(r.style.Align(lipgloss.Right).Faint(true).Render("motion\nsensors")),
			lipgloss.JoinVertical(lipgloss.Left, sensorsList...),
		),
	)

	shortRoomName := strings.ReplaceAll(r.Name, "room", "")
	height := int(math.Max(float64(len(daytimesList)), float64(len(lightsList)+len(sensorsList)+1)) + 2)

	out.WriteString(
		lipgloss.JoinHorizontal(
			lipgloss.Top,
			lipgloss.NewStyle().
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(r.color).
				BorderRight(true).
				PaddingRight(1).
				Align(lipgloss.Right).
				AlignVertical(lipgloss.Center).
				Height(height).
				Width(8).
				Render(r.style.Render(shortRoomName)),
			lipgloss.NewStyle().PaddingTop(1).Width(48).Render(lipgloss.JoinVertical(lipgloss.Left, daytimesList...)),
			lipgloss.JoinVertical(
				lipgloss.Left,
				fmtLightsList,
				fmtSensorsList,
			),
		),
	)

	return lipgloss.NewStyle().MarginLeft(1).Render(out.String())
}

// eventReceiver listens for events on the room's event channel and forwards them to the event handler.
func (r *Room) eventReceiver() {
	r.pr.Info("event receiver started")

	for event := range r.EventsChannel {
		r.pr.Debugf("received event: %+v", event)

		// handle event in a new goroutine to prevent blocking the event receiver
		go r.eventHandler(event)
	}
}

func (r *Room) scheduleDaytimeSwitches() {
	for _, dt := range r.Daytimes {
		_, err := (*r.aml.daytimeSwitcher).Every(1).Day().At(dt.Start.UTC().Format("15:04")).Tag(r.Name).Tag(dt.Name).Do(r.switchDaytime, dt)
		if err != nil {
			r.pr.Errorf("❌ scheduling job failed: %+v", err)
		}
	}
}

func (r *Room) switchDaytime(daytime *daytime.Daytime) {
	r.pr.Debugf("%s daytime switch to: %+v", icons.Alarm, daytime)

	// set new active daytime
	r.activeDaytimeIndex = slices.Index(r.Daytimes, daytime)
	actionDone := "set to"
	divider := style.DarkIndicatorRight

	// // optional immediate transition to new daytime
	// if room.transitionOnDaytimeSwitch {
	// 	actionDone = "activated"
	// 	divider = style.DarkIndicatorRight.Copy().Foreground(room.color)
	// 	// TODO transition to daytime
	// }

	// build daytime switch message
	daytimeSwitchMsg := strings.Builder{}
	daytimeSwitchMsg.WriteString(icons.Alarm)
	daytimeSwitchMsg.WriteString(" daytime " + actionDone + " ")
	daytimeSwitchMsg.WriteString(style.Bold(daytime.Name))
	daytimeSwitchMsg.WriteString(" " + divider.String() + " ")
	daytimeSwitchMsg.WriteString(r.FormatDaytimeConfiguration(daytime))

	r.pr.Print(daytimeSwitchMsg.String())
}

func (r *Room) eventHandler(event *homeassistant.EventMsg) {
	r.pr.Debugf("handling event: %+v", event)

	entityID := event.Event.Data.EntityID
	eventType := event.Event.Type
	friendlyName := r.aml.ha.FriendlyName(entityID)

	// count events
	r.eventsReceivedTotal.Add(1)

	// filter out irrelevant state changes
	if eventType == homeassistant.EventStateChanged && (r.MotionStateOn == "" || r.MotionStateOn != event.Event.Data.NewState.State) {
		r.pr.Debugf("%s ignoring %s to non-trigger state %s | ←%s %s %s", icons.Blind, style.Bold(string(eventType)), style.Bold(event.Event.Data.NewState.State), friendlyName, style.DarkDivider.String(), event.Event.Data.EntityID.FmtShort())
		r.pr.Debugf("%+v", pretty.Sprint(event.Event))

		return
	}

	//
	// → valid motion event
	r.pr.Debugf(
		"%s received %s | %s%s %s %s", icons.Trigger, style.Bold(string(eventType)), style.DarkIndicatorLeft, friendlyName, style.DarkDivider.String(), entityID.FmtShort(),
	)

	// refresh the timer after valid motion event
	r.refreshTimer()

	// lock the room to prevent concurrent access
	r.Lock()
	defer r.Unlock()

	// check if the conditions to turn on the lights are fulfilled
	if ok, err := r.canTurnOnLights(); !ok {
		r.pr.Infof("%s %s | %s", icons.Block, service.TurnOn.FmtStringStriketrough(), err)

		return
	}

	// checks passed - turn on the lights 💡
	_ = r.turnLightsOn(event)

	// message about the trigger event
	triggerMsg := strings.Builder{}
	triggerMsg.WriteString(icons.Motion + " ")
	triggerMsg.WriteString(style.Bold(string(eventType)) + " ")
	triggerMsg.WriteString(style.DarkDivider.String() + " ")
	triggerMsg.WriteString(style.DarkIndicatorLeft.String())
	triggerMsg.WriteString(friendlyName + " ")
	triggerMsg.WriteString(event.Event.Data.EntityID.FmtShort())

	r.pr.Info(triggerMsg.String())
}

// canTurnOnLights checks if all conditions to turn on the lights are fulfilled.
func (r *Room) canTurnOnLights() (bool, error) {
	switch {
	// check if the room/AutoMoLi in general is disabled
	case r.aml.isDisabled():
		return false, fmt.Errorf("%w: %+v", models.ErrAutoMoLiDisabled, strings.Join(r.fmtDisabler(), " | "))

	// check if the lights are disabled by the current daytime/light configuration
	case r.isDisabledByLightConfiguration():
		return false, fmt.Errorf("%w: %+v", models.ErrDaytimeDisabled, r.GetActiveDaytime())

	// check if the lights are already on
	case r.isLightOn():
		return false, fmt.Errorf("%w: %+v", models.ErrLightAlreadyOn, r.lightsOn())

	// check if the lights were just turned on (but it may have been not recognized yet)
	case time.Since(r.lastSwitchedOn) < viper.GetDuration("automoli.defaults.relax_after_turn_on"):
		return false, fmt.Errorf("%w: %+v", models.ErrLightJustTurnedOn, time.Since(r.lastSwitchedOn))
	}

	return true, nil
}
