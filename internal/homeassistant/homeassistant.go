package homeassistant

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/benleb/automoli-go/internal/icons"
	"github.com/benleb/automoli-go/internal/models"
	"github.com/benleb/automoli-go/internal/models/service"
	"github.com/benleb/automoli-go/internal/style"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/mitchellh/mapstructure"
)

type HomeAssistant struct {
	URL   *url.URL `json:"url" yaml:"url"`
	Token string   `json:"-"   yaml:"-"`

	// holds the current state of all entities and is updated on state_changed events
	states   map[EntityID]*State
	statesMu sync.RWMutex

	eventChannel   chan *EventMsg
	resultsHandler map[int64]*chan ResultMsg

	// desired subscriptions
	subscriptions mapset.Set[EventType]
	// actually active subscriptions
	activeSubscriptions mapset.Set[EventType]

	pr *log.Logger

	Conn  *websocket.Conn
	nonce atomic.Int64

	receivedMsgs atomic.Uint64

	sync.RWMutex
	startTime time.Time

	lastMessageReceived time.Time
}

func (ha *HomeAssistant) wsURL() string {
	wsURL := *ha.URL

	switch ha.URL.Scheme {
	case "http":
		wsURL.Scheme = "ws"
	case "https":
		wsURL.Scheme = "wss"

	default:
		ha.pr.Errorf("unsupported scheme: %s", ha.URL.Scheme)
	}

	return wsURL.JoinPath("/api/websocket").String()
}

func New(rawURL string, token string) (*HomeAssistant, error) {
	haURL, err := url.Parse(rawURL)
	if err != nil {
		log.Fatal(err)
	}

	hass := &HomeAssistant{
		URL:   haURL,
		Token: token,

		nonce: atomic.Int64{},

		states: make(map[EntityID]*State),

		resultsHandler: make(map[int64]*chan ResultMsg),

		subscriptions:       mapset.NewSet[EventType](),
		activeSubscriptions: mapset.NewSet[EventType](),

		pr: models.Printer.WithPrefix(lipgloss.NewStyle().Foreground(style.HABlue).Render("HA")),

		startTime: time.Now(),
	}

	// initial connect & authenticate to websockets API
	if err := hass.connectAndAuthenticate(); err != nil {
		hass.pr.Errorf("failed to connect to server: %+v", err)

		return nil, err
	}

	hass.pr.Infof("%s connected to %s", icons.ConnectionChain, haBlueFrame(hass.URL.String()))

	// start message handler
	go hass.wsReader()

	// subscribe to state_changed events
	hass.subscribe(EventStateChanged)

	hass.pr.Infof("%s subscribed to %s", icons.Sub, haBlueFrame(string(EventStateChanged)))

	// get initial state
	hass.getStates()

	hass.pr.Infof("%s got initial state: %d entities", icons.Home, len(hass.states))

	return hass, nil
}

func (ha *HomeAssistant) Attributes(entityID EntityID) *Attributes {
	state := ha.GetState(entityID)
	if state == nil {
		return nil
	}

	return &state.Attributes
}

func (ha *HomeAssistant) FriendlyName(entityID EntityID) string {
	state := ha.GetState(entityID)
	if state == nil {
		return ""
	}

	return state.Attributes.FriendlyName
}

func (ha *HomeAssistant) GetState(entityID EntityID) *State {
	ha.statesMu.RLock()
	state, ok := ha.states[entityID]
	ha.statesMu.RUnlock()

	if !ok || state == nil {
		ha.pr.Errorf("no state found for entity %s in %d states", entityID.ID, len(ha.states))

		return nil
	}

	return state
}

func (ha *HomeAssistant) subscribe(eventType EventType) {
	// already subscribed?
	if ha.activeSubscriptions.Contains(eventType) {
		ha.pr.Info(
			icons.Sub + " " + icons.GreenTick.Render() +
				style.LightGray.Render(" already subscribed to ") +
				style.Bold(string(eventType)) +
				style.LightGray.Render(" events"),
		)

		return
	}

	// add to subscriptions list for re-subscribing after reconnect
	ha.subscriptions.Add(eventType)
	ha.activeSubscriptions.Add(eventType)

	// create ws message
	if ha.wsCall(nil, NewSubscribeMsg(eventType)) == 0 {
		ha.pr.Warnf("❌ subscription for %+v failed", style.Bold(string(eventType)))
	}
}

func (ha *HomeAssistant) SubscribeToEvents(eventType EventType, eventChannel *chan *EventMsg) {
	ha.subscribe(eventType)
	ha.eventChannel = *eventChannel
}

func (ha *HomeAssistant) reconnect() {
	failCount := 0

	for {
		if err := ha.connectAndAuthenticate(); err != nil {
			// exponential backoff
			sleepInSeconds := int(math.Min(8.0, math.Pow(2, float64(failCount))))

			failCount++

			ha.pr.Errorf(
				"%s connection failed (%d) %s retry in %ds %s %+v",
				icons.ConnectionFailed,
				failCount,
				lipgloss.NewStyle().Foreground(style.HABlue).Render("|"),
				sleepInSeconds,
				lipgloss.NewStyle().Foreground(lipgloss.Color("#999")).Render("|"),
				err,
			)

			time.Sleep(time.Duration(sleepInSeconds) * time.Second)
		} else {
			ha.pr.Print(icons.ConnectionOK + " reconnected")

			ha.lastMessageReceived = time.Now()
			ha.activeSubscriptions.Clear()

			// re-subscribe to events
			for _, eventType := range ha.subscriptions.ToSlice() {
				ha.subscribe(eventType)
			}

			ha.pr.Printf("%s re-subscribed to %d events", icons.ConnectionOK, ha.subscriptions.Cardinality())

			break
		}
	}
}

// LastMessageReceivedWatchdog checks if the last message received is older than 10s and reconnects if so.
func (ha *HomeAssistant) LastMessageReceivedWatchdog(maxAge, checkEvery time.Duration) {
	ha.pr.Infof("%s starting last message received watchdog | max age: %s | check every: %s", icons.Watchdog, style.Bold(maxAge.String()), style.Bold(checkEvery.String()))

	for {
		time.Sleep(checkEvery)

		since := time.Since(ha.lastMessageReceived)
		if since > maxAge {
			ha.pr.Warnf("❌ no message received for %s - reconnecting", style.Bold(time.Since(ha.lastMessageReceived).String()))

			// reconnect
			ha.reconnect()

			continue
		}

		ha.pr.Debugf("%s %s last message received %s ago | max age: %s | next check: %s", icons.Watchdog, icons.GreenTick.Render(), style.Bold(since.Round(time.Millisecond).String()), style.Bold(maxAge.String()), style.Bold(checkEvery.String()))
	}
}

func (ha *HomeAssistant) wsReader() {
	ha.pr.Infof("%s starting message handler", icons.WeightLift)

	for {
		// read message from websocket
		if ha.Conn == nil {
			ha.pr.Error("no connection to server")

			// reconnect
			ha.reconnect()
		}

		var msg map[string]interface{}

		err := wsjson.Read(context.TODO(), ha.Conn, &msg)
		if err != nil {
			ha.pr.Errorf("🚘 failed to read message: %+v", err)

			if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
				ha.pr.Error("server closed with StatusNormalClosure - might be restarting ✊")
			}

			// reconnect
			ha.reconnect()

			continue
		}

		ha.receivedMsgs.Add(1)
		ha.lastMessageReceived = time.Now()

		if msg == nil {
			ha.pr.Error("received nil message")

			continue
		}

		msgType, ok := msg["type"].(string)
		if !ok {
			ha.pr.Error("received message without type")

			continue
		}

		switch msgType {
		case "event":
			ha.handleEventMessage(msg)
		case "result":
			ha.handleResultMessage(msg)
		}
	}
}

func (ha *HomeAssistant) handleEventMessage(msg map[string]interface{}) {
	var eventMsg EventMsg
	var metadata *mapstructure.Metadata

	// map msg to eventMsg
	decodeHooks := mapstructure.ComposeDecodeHookFunc(mapstructure.StringToTimeHookFunc(time.RFC3339), StringToEntityIDHookFunc())

	decoder, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: decodeHooks,
		Result:     &eventMsg,
		Metadata:   metadata,
	})

	if err := decoder.Decode(msg); err != nil {
		ha.pr.Errorf("decoding incoming event failed: %+v", err)
	}

	// update local state
	if eventMsg.Event.Type == EventStateChanged {
		// update local state
		go ha.updateStates([]*State{&eventMsg.Event.Data.NewState})

		ha.pr.Debugf("✔️ updated state for %s: %+v", eventMsg.Event.Data.EntityID.ID, ha.GetState(eventMsg.Event.Data.EntityID))
	}

	// only forward subscribed events
	if ha.subscriptions.Contains(eventMsg.Event.Type) {
		ha.eventChannel <- &eventMsg
	} else {
		ha.pr.Warnf("❔ received unexpected %s event: %+v | expected events: %+v", style.Bold(string(eventMsg.Event.Type)), eventMsg, style.Bold(ha.subscriptions.String()))

		return
	}
}

func (ha *HomeAssistant) handleResultMessage(msg map[string]interface{}) {
	var resultMsg ResultMsg

	var metadata *mapstructure.Metadata

	// map msg to resultMsg
	decodeHooks := mapstructure.ComposeDecodeHookFunc(mapstructure.StringToTimeHookFunc(time.RFC3339), StringToEntityIDHookFunc())

	decoder, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: decodeHooks,
		Result:     &resultMsg,
		Metadata:   metadata,
	})

	if err := decoder.Decode(msg); err != nil {
		ha.pr.Error("decoding incoming event failed:", err)

		return
	}

	if !resultMsg.Success {
		ha.pr.Errorf(style.Gray(6).Render("#")+"%d | %s | %s", resultMsg.ID, resultMsg.Error.Code, resultMsg.Error.Message)

		return
	}

	// is there a result (= data) to handle or just a success message?
	if ha.resultsHandler[resultMsg.ID] != nil && resultMsg.Result != nil {
		*ha.resultsHandler[resultMsg.ID] <- resultMsg
	}
}

func (ha *HomeAssistant) TurnOn(targets []EntityID, serviceData map[string]interface{}) mapset.Set[*ResultMsg] {
	return ha.turnOnOff(targets, service.TurnOn, serviceData)
}

func (ha *HomeAssistant) TurnOff(targets []EntityID, serviceData map[string]interface{}) mapset.Set[*ResultMsg] {
	return ha.turnOnOff(targets, service.TurnOff, serviceData)
}

func (ha *HomeAssistant) turnOnOff(targets []EntityID, haService service.Service, serviceData map[string]interface{}) mapset.Set[*ResultMsg] {
	waitGroup := sync.WaitGroup{}

	results := mapset.NewSet[*ResultMsg]()

	for _, target := range targets {
		waitGroup.Add(1)

		filteredServiceData := filterServiceData(serviceData, models.AllowedServiceData[haService][target.Domain()])

		go func(target EntityID) {
			// call service
			result := ha.wsCallWithResponse(NewCallServiceMsg(haService, filteredServiceData, target))
			if result == nil {
				ha.pr.Warnf("call(s) failed | %s for %s: %+v", haService, target.ID, result)

				waitGroup.Done()

				return
			}

			results.Add(result)

			if result.Success {
				// update local state
				if target.Domain() == "light" || target.Domain() == "switch" {
					ha.updateStateValue(target, strings.TrimPrefix(haService.String(), "turn_"))
				}

				ha.pr.Infof("%s %s %s", icons.Call, result, icons.GreenTick.String())
			} else {
				ha.pr.Warnf("%s %s %s", icons.Call, result, icons.RedCross.String())
			}

			waitGroup.Done()
		}(target)
	}

	waitGroup.Wait()

	return results
}

// filterServiceData filters the given service data map by the allowed keys.
func filterServiceData(serviceData map[string]interface{}, allowedKeys mapset.Set[string]) map[string]interface{} {
	if allowedKeys == nil {
		return make(map[string]interface{})
	}

	filteredServiceData := make(map[string]interface{})

	for key, value := range serviceData {
		if allowedKeys.Contains(key) {
			filteredServiceData[key] = value
		} else {
			log.Warnf("❗️ removing not allowed service data key: %s", key)
		}
	}

	return filteredServiceData
}

func (ha *HomeAssistant) wsCallWithResponse(msg Message) *ResultMsg {
	// create response channel
	done := make(chan ResultMsg, 1)

	// send message and wait for result
	msgID := ha.wsCall(&done, msg)
	result := <-done

	// remove result handler
	delete(ha.resultsHandler, msgID)

	return &result
}

// wsCall sends a message to the websocket connection and returns the used message id.
func (ha *HomeAssistant) wsCall(done *chan ResultMsg, msg Message) int64 {
	// send message with increasing unique message id
	ha.Lock()
	defer ha.Unlock()

	// add unique message id
	msgID := msg.SetID(ha.nonce.Add(1))

	// optionally add a result handler
	if done != nil {
		ha.resultsHandler[msgID] = done
	}

	// send the message
	if err := wsjson.Write(context.TODO(), ha.Conn, msg); err != nil {
		ha.pr.Error(fmt.Errorf("failed to write message: %w", err))

		return -1
	}

	// ha.pr.Printf("🆔 sent msg with id: %d", msgID)

	return msgID
}

func (ha *HomeAssistant) getStates() {
	// create ws message
	msg := &baseMessage{Type: "get_states"}
	done := make(chan ResultMsg, 1)

	// send message and wait for result
	msgID := ha.wsCall(&done, msg)
	result := <-done

	// remove result handler
	delete(ha.resultsHandler, msgID)

	// map result to State structs
	var states []*State
	var metadata *mapstructure.Metadata

	decoder, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(mapstructure.StringToTimeHookFunc(time.RFC3339), StringToEntityIDHookFunc()),
		Result:     &states,
		Metadata:   metadata,
	})

	// decode result
	err := decoder.Decode(result.Result)
	if err != nil {
		ha.pr.Error("❌ decoding incoming get_states result failed:", err)

		return
	}

	// check if we received any states
	if len(states) == 0 {
		ha.pr.Error("❌ no states received")

		return
	}

	// update local state
	ha.updateStates(states)
}

// updateStates updates the local state with the given states.
func (ha *HomeAssistant) updateStates(states []*State) {
	ha.statesMu.Lock()
	defer ha.statesMu.Unlock()

	for _, state := range states {
		ha.states[state.EntityID] = state
	}
}

// updateStates updates the local state with the given states.
func (ha *HomeAssistant) updateStateValue(target EntityID, state string) {
	ha.statesMu.Lock()
	defer ha.statesMu.Unlock()

	if state == "" {
		ha.pr.Debugf("❌ empty state for %s", target.ID)

		return
	}

	ha.states[target].State = state
}

// connectAndAuthenticate connects to the websocket API and handles the authentication.
func (ha *HomeAssistant) connectAndAuthenticate() error {
	// ensure existing connection is closed
	if ha.Conn != nil {
		ha.pr.Info("❌ closing existing connection... %+v", ha.Conn)

		if err := ha.Conn.Close(websocket.StatusNormalClosure, "reconnecting"); err != nil {
			ha.pr.Errorf("❌ failed to close connection: %+v", err)

			// force close
			if ha.Conn != nil {
				ha.pr.Info("❌ force closing existing connection... %#v", ha.Conn)

				_ = ha.Conn.CloseNow()
			} else {
				ha.pr.Info("❌ no connection to close")
			}
		}

		// ha.Conn = nil
	}

	ha.Conn = nil

	// create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	// connect to websocket API
	conn, _, err := websocket.Dial(ctx, ha.wsURL(), &websocket.DialOptions{})
	if err != nil {
		return err
	}

	// set max size of a message in bytes
	conn.SetReadLimit(1024000) // 1024kb

	// authenticate
	if err := ha.authenticate(conn, ha.Token); err != nil {
		ha.pr.Error(fmt.Errorf("failed to authenticate: %w", err))

		return err
	}

	ha.pr.Info("🔑 successfully authenticated")

	// set new connection
	ha.Conn = conn

	ha.lastMessageReceived = time.Now()
	ha.activeSubscriptions.Clear()

	return nil
}

var errUnexpectedMessageType = errors.New("unexpected message type")

func unexpectedMsgType(msgType string) error {
	return fmt.Errorf("%w: %s", errUnexpectedMessageType, msgType)
}

// authenticate authenticates to the websocket API.
func (ha *HomeAssistant) authenticate(conn *websocket.Conn, token string) error {
	// authenticate
	var versionMsg VersionMsg

	// read first message...
	err := wsjson.Read(context.TODO(), conn, &versionMsg)
	if err != nil {
		ha.pr.Error(fmt.Errorf("failed to read message: %w", err))

		return err
	}

	// ...which should be the auth_required message
	if versionMsg.Type != "auth_required" {
		ha.pr.Error(unexpectedMsgType(versionMsg.Type))

		return err
	}

	// reply with auth message containing a token
	err = wsjson.Write(context.TODO(), conn, NewAuthMsg(token))
	if err != nil {
		ha.pr.Error(fmt.Errorf("failed to write message: %w", err))

		return err
	}

	err = wsjson.Read(context.TODO(), conn, &versionMsg)
	if err != nil {
		ha.pr.Error(fmt.Errorf("failed to read message: %w", err))

		return err
	}

	if versionMsg.Type != "auth_ok" {
		ha.pr.Error(unexpectedMsgType(versionMsg.Type))

		return err
	}

	// update counter
	// ha.sentMsgs.Add(1)
	ha.receivedMsgs.Add(2)

	return nil
}

func haBlue(text string) string {
	// return style.HAStyle.Copy().SetString(text).Render()
	return style.HAStyle.SetString(text).Render()
}

func haBlueFrame(text string) string {
	return haBlue("<") + text + haBlue(">")
}
