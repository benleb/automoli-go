package homeassistant

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/benleb/automoli-go/internal/icons"
	"github.com/benleb/automoli-go/internal/models"
	"github.com/benleb/automoli-go/internal/models/domain"
	"github.com/benleb/automoli-go/internal/models/service"
	"github.com/benleb/automoli-go/internal/style"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

var (
	connectionTimeout = time.Second * 5
	reconnectDelay    = 7 * time.Second
	readLimit         = int64(1024000) // 1024kb
)

type HomeAssistant struct {
	wsURL   *url.URL
	httpURL *url.URL
	token   string

	// holds the current state of all entities and is updated on state_changed events
	states   map[EntityID]*State
	statesMu sync.RWMutex

	// events received from the websocket connection
	receivedEvents chan *EventMsg
	// time the most recent event was received
	lastEventReceived time.Time
	lastEventTicker   *time.Ticker

	// map of the result handlers for sent messages/requests
	resultsHandler map[int64]*chan ResultMsg

	// desired subscriptions
	subscriptions mapset.Set[EventType]
	// actually active subscriptions
	activeSubscriptions mapset.Set[EventType]

	// printer
	pr *log.Logger

	// websocket connection
	conn *websocket.Conn
	// lock for the websocket
	wsMutex sync.Mutex

	// time of start
	startTime time.Time
}

// New creates a new HomeAssistant instance and connects to the websocket API.
func New(rawURL string, token string, eventsChannel *chan *EventMsg) (*HomeAssistant, error) {
	// create new HomeAssistant instance
	haClient, err := createHomeAssistantInstance(rawURL, token, eventsChannel)
	if err != nil {
		return nil, err
	}

	haClient.setup()

	haClient.pr.Printf("%s Home Assistant client started", icons.GreenTick)

	return haClient, nil
}

func createInstance(rawURL string, token string, eventsChannel *chan *EventMsg) (*HomeAssistant, error) {
	// validity check
	if rawURL == "" {
		return nil, models.ErrEmptyURL
	} else if token == "" {
		return nil, models.ErrEmptyToken
	}

	// parse http(s) URL
	httpURL, err := url.Parse(rawURL)
	if err != nil {
		log.Fatal("failed to parse URL: ", err)
	}

	// create websocket URL
	wsURL := *httpURL
	switch httpURL.Scheme {
	case "http":
		wsURL.Scheme = "ws"
	case "https":
		wsURL.Scheme = "wss"
	default:
		log.Errorf("unsupported url scheme: %s", httpURL.Scheme)
	}

	// create new HomeAssistant instance
	homAss := &HomeAssistant{
		wsURL:   wsURL.JoinPath("/api/websocket"),
		httpURL: httpURL,
		token:   token,

		states: make(map[EntityID]*State),

		receivedEvents: *eventsChannel,

		lastEventReceived: time.Now(),
		lastEventTicker:   time.NewTicker(viper.GetDuration("homeassistant.defaults.watchdog_check_every")),

		nonce: atomic.Int64{},

		resultsHandler: make(map[int64]*chan ResultMsg),

		// events we always want to subscribe to
		subscriptions:       mapset.NewSet(EventStateChanged, EventHomeAssistantStart, EventHomeAssistantStarted),
		activeSubscriptions: mapset.NewSet[EventType](),

		pr: models.Printer.WithPrefix(lipgloss.NewStyle().Foreground(style.HABlue).Render("HA")),

		startTime: time.Now(),
	}

	return homAss, nil
}

// setup sets up the HomeAssistant client to receive events.
func (ha *HomeAssistant) setup() {
	initialSetup := true

	// shutdown current connection
	if ha.conn != nil {
		initialSetup = false

		ha.pr.Infof("%s reconnect - closing existing connection...", icons.Stopwatch)

		// reconnect - tear down existing client
		ha.shutdown()
	}

	for {
		if !initialSetup {
			ha.pr.Printf("%s trying again in %.0fs...", icons.ReconnectCircle, reconnectDelay.Seconds())
			time.Sleep(reconnectDelay)
		}

		// setup
		if err := ha.setupConnection(); err != nil {
			ha.pr.With("err", err).Error("failed to setup connection")

			continue
		}

		// subscribe to events
		if err := ha.setupSubscriptions(); err != nil {
			ha.pr.With("err", err).Error("failed to setup subscriptions")

			continue
		}

		// start watchdog for last event received
		go ha.lastEventReceivedWatchdog(viper.GetDuration("homeassistant.defaults.watchdog_max_age"), viper.GetDuration("homeassistant.defaults.watchdog_check_every"))

		// success
		break
	}

	if !initialSetup {
		ha.pr.Printf("%s reconnected", icons.ReconnectCircle)
	}
}

func (ha *HomeAssistant) setupConnection() error {
	// connect to websocket API
	ha.pr.Printf("%s connecting to %s", icons.ConnectionChain, ha.wsURL.String())

	// create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, ha.wsURL.String(), &websocket.DialOptions{})
	if err != nil {
		return err
	}

	ha.conn = conn

	ha.pr.Printf("%s connected to %s", icons.GreenTick, ha.wsURL.String())

	// increase max size of a message for the connection (in bytes)
	ha.conn.SetReadLimit(readLimit)

	ha.pr.Printf("%s set read limit to %d bytes", icons.Glasses, readLimit)

	// authenticate
	if err := ha.doAuthentication(); err != nil {
		ha.pr.Error("authentication failed: ", err)

		return err
	}

	ha.pr.Printf("%s successfully authenticated", icons.Key)

	return nil
}

func (ha *HomeAssistant) setupSubscriptions() error {
	// start message handler
	go ha.runReader()

	// get initial state
	numStatesReceived, err := ha.getStates()
	if err != nil {
		ha.pr.Error("failed to get states: ", err)

		return err
	} else if numStatesReceived == 0 {
		ha.pr.Error("no states received")

		return models.ErrNoStatesReceived
	}

	ha.pr.Printf("%s fetched states for %d entities", icons.Home, numStatesReceived)

	// subscribe
	eventsNotSubscribed := ha.subscriptions.Difference(ha.activeSubscriptions)
	ha.pr.Printf("%s subscribing to %d events: %+v", icons.Sub, ha.subscriptions.Cardinality(), eventsNotSubscribed)

	ha.subscribe()

	return nil
}

func (ha *HomeAssistant) shutdown() {
	// stop last event received watchdog
	ha.lastEventTicker.Stop()

	// try graceful close of the existing connection
	if ha.conn != nil {
		ha.pr.Debugf("%s closing existing connection... %+v", icons.RedCross.Render(), ha.conn)

		if err := ha.conn.Close(websocket.StatusNormalClosure, "reconnect"); err != nil {
			ha.pr.Debugf("%s failed to gracefully close connection: %+v", icons.RedCross.Render(), err)

			// force close
			if ha.conn != nil {
				ha.pr.Debugf("🤷%s force closing the connection... %#v", icons.Shrug, ha.conn)

				_ = ha.conn.CloseNow()
			}
		}
	}

	// close results handler
	for _, done := range ha.resultsHandler {
		close(*done)
	}

	// clear active subscriptions
	ha.activeSubscriptions.Clear()

	// clear states
	ha.states = make(map[EntityID]*State)

	// clear results handler
	ha.resultsHandler = make(map[int64]*chan ResultMsg)

	// clear websocket connection
	ha.conn = nil

	// clear nonce
	ha.nonce.Store(1337)
}

// authenticate authenticates to the websocket API.
func (ha *HomeAssistant) doAuthentication() error {
	// authenticate
	var versionMsg VersionMsg

	// read first message...
	err := wsjson.Read(context.TODO(), ha.conn, &versionMsg)
	if err != nil {
		ha.pr.Error(fmt.Errorf("failed to read message: %w", err))

		return err
	}

	// ...which should be the auth_required message
	if versionMsg.Type != "auth_required" {
		return models.ErrUnexpectedMessageType
	}

	// reply with auth message containing a token
	err = wsjson.Write(context.TODO(), ha.conn, NewAuthMsg(ha.token))
	if err != nil {
		ha.pr.Error(fmt.Errorf("failed to write message: %w", err))

		return err
	}

	err = wsjson.Read(context.TODO(), ha.conn, &versionMsg)
	if err != nil {
		ha.pr.Error(fmt.Errorf("failed to read message: %w", err))

		return err
	}

	if versionMsg.Type != "auth_ok" {
		ha.pr.Error(fmt.Errorf("%w: %s", models.ErrUnexpectedMessageType, versionMsg.Type))

		return err
	}

	return nil
}

func (ha *HomeAssistant) GetState(entityID EntityID) *State {
	ha.statesMu.RLock()
	state, ok := ha.states[entityID]
	ha.statesMu.RUnlock()

	if !ok {
		ha.pr.Warnf("entity %s not found in %d states", entityID.ID, len(ha.states))

		return nil
	} else if state == nil {
		ha.pr.Warnf("no state found for entity %s in %d states", entityID.ID, len(ha.states))

		return nil
	}

	return state
}

func (ha *HomeAssistant) FriendlyName(entityID EntityID) string {
	state := ha.GetState(entityID)
	if state == nil {
		return ""
	}

	return state.Attributes.FriendlyName
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
			result, err := ha.wsCallWithResponse(NewCallServiceMsg(haService, filteredServiceData, target))
			if result == nil || err != nil {
				ha.pr.Warnf("call(s) failed | %s for %s: %+v ||| %+v", haService, target.ID, result, err)

				waitGroup.Done()

				return
			}

			results.Add(result)

			if result.Success {
				// update local state
				if target.Domain() == domain.Light || target.Domain() == domain.Switch {
					ha.updateStateValue(target, strings.TrimPrefix(haService.String(), "turn_"))
				}

				ha.pr.Debugf("%s %s %s", icons.Call, result, icons.GreenTick.String())
			} else {
				ha.pr.Warnf("%s %s %s", icons.Call, result, icons.RedCross.String())
			}

			waitGroup.Done()
		}(target)
	}

	waitGroup.Wait()

	return results
}

// SubscribeToEvent adds the given event to the subscriptions list and subscribes to it.
func (ha *HomeAssistant) SubscribeToEvent(subscriptionEvent EventType) {
	ha.SubscribeToEvents(mapset.NewSet[EventType](subscriptionEvent))
}

// SubscribeToEvents adds the given events to the subscriptions list and subscribes to them.
func (ha *HomeAssistant) SubscribeToEvents(subscriptionEvents mapset.Set[EventType]) {
	// subscribe to events
	for eventType := range subscriptionEvents.Iter() {
		// add to subscriptions list
		ha.subscriptions.Add(eventType)
	}

	// subscribe to events
	ha.subscribe()
}

func (ha *HomeAssistant) subscribe() {
	// get events not subscribed to yet
	eventsNotSubscribed := ha.subscriptions.Difference(ha.activeSubscriptions)

	// subscribe to events
	for eventType := range eventsNotSubscribed.Iter() {
		if _, err := ha.wsCall(nil, NewSubscribeMsg(eventType)); err != nil {
			ha.pr.Warnf("❌ subscription for %+v failed: %s", style.Bold(string(eventType)), err)
		} else {
			ha.pr.Infof("%s subscribed to %s", icons.Sub, style.HABlueFrame(string(eventType)))

			// add to active subscriptions
			ha.activeSubscriptions.Add(eventType)
		}
	}
}

func (ha *HomeAssistant) wsCallWithResponse(msg Message) (*ResultMsg, error) {
	// create response channel
	done := make(chan ResultMsg, 1)

	// send message and wait for result
	msgID, err := ha.wsCall(&done, msg)
	if err != nil {
		ha.pr.Error(fmt.Errorf("failed to send message: %w", err))

		return nil, err
	}

	result := <-done

	// remove result handler
	delete(ha.resultsHandler, msgID)

	return &result, nil
}

// wsCall sends a message to the websocket connection and returns the used message id.
func (ha *HomeAssistant) wsCall(done *chan ResultMsg, msg Message) (int64, error) {
	// send message with increasing unique message id
	ha.wsMutex.Lock()
	defer ha.wsMutex.Unlock()

	// add unique message id
	msgID := msg.SetID(ha.nonce.Add(1))

	// optionally add a result handler
	if done != nil {
		ha.resultsHandler[msgID] = done
	}

	if ha.conn == nil {
		return 0, models.ErrNoConnectionToWriteTo
	}

	// send the message
	if err := wsjson.Write(context.Background(), ha.conn, msg); err != nil {
		return 0, err
	}

	return msgID, nil
}

func (ha *HomeAssistant) getStates() (int, error) {
	// create ws message
	msg := &baseMessage{Type: "get_states"}
	done := make(chan ResultMsg, 1)

	// send message and wait for result
	msgID, err := ha.wsCall(&done, msg)
	if err != nil {
		ha.pr.Error(fmt.Errorf("failed to get states: %w", err))

		return 0, err
	}
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
	err = decoder.Decode(result.Result)
	if err != nil {
		ha.pr.Error("❌ decoding incoming get_states result failed:", err)

		return 0, err
	}

	numStates := len(states)

	// check if we received any states
	if numStates == 0 {
		ha.pr.Error("❌ no states received")

		return 0, models.ErrNoStatesReceived
	}

	// update local state
	ha.updateStates(states)

	return numStates, nil
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

func (ha *HomeAssistant) runReader() {
	ha.pr.Printf("%s starting websocket reader", icons.WeightLift)

	if err := ha.wsReader(); err != nil {
		ha.pr.Errorf("%s reader error: %+v", icons.Glasses, err)

		// shutdown & reconnect
		go ha.setup()

		return
	}
}

func (ha *HomeAssistant) wsReader() error {
	for {
		// read message from websocket
		if ha.conn == nil {
			return models.ErrNoConnectionToReadFrom
		}

		var msg map[string]interface{}

		err := wsjson.Read(context.TODO(), ha.conn, &msg)
		if err != nil {
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
				return models.ErrConnectionClosed
			}

			return err
		}

		if msg == nil {
			ha.pr.Error("received nil message")

			continue
		}

		msgType, ok := msg["type"].(string)
		if !ok {
			ha.pr.Errorf("received message without type: %+v", msg)

			continue
		}

		switch msgType {
		case "event":
			ha.handleEventMessage(msg)
		case "result":
			ha.handleResultMessage(msg)

		default:
			ha.pr.Warnf("❔ received unexpected %s message: %+v", style.Bold(msgType), msg)
		}

		ha.lastEventReceived = time.Now()
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
		ha.pr.Errorf("decoding incoming event failed: %+v | msg: %+v", err, msg)

		return
	}

	switch {
	// update local state
	case eventMsg.Event.Type == EventStateChanged:
		ha.updateStates([]*State{&eventMsg.Event.Data.NewState})

		ha.pr.Debugf("%s updated state for %s: %+v", icons.Tick, eventMsg.Event.Data.EntityID.ID, ha.GetState(eventMsg.Event.Data.EntityID))

	// only forward subscribed events
	case ha.subscriptions.Contains(eventMsg.Event.Type):
		ha.receivedEvents <- &eventMsg

	// home assistant start
	case eventMsg.Event.Type == EventHomeAssistantStart || eventMsg.Event.Type == EventHomeAssistantStarted:
		ha.pr.Printf("🚀 %s received", style.Bold(string(eventMsg.Event.Type)))

		// get states
		_, err := ha.getStates()
		if err != nil {
			ha.pr.Error("failed to get states: ", err)

			return
		}

	// received unexpected event
	default:
		ha.pr.Printf("❔ received unexpected %s event: %+v | expected events: %+v", style.Bold(string(eventMsg.Event.Type)), eventMsg, style.Bold(ha.subscriptions.String()))
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
		ha.pr.Errorf("decoding incoming event failed: %+v | msg: %+v", err, msg)

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

// lastEventReceivedWatchdog checks if the last event received is older than the given max age.
func (ha *HomeAssistant) lastEventReceivedWatchdog(maxAge, checkEvery time.Duration) {
	ha.pr.Infof("%s starting last event received watchdog | max age: %s | check every: %s", icons.Watchdog, style.Bold(maxAge.String()), style.Bold(checkEvery.String()))

	for range ha.lastEventTicker.C {
		since := time.Since(ha.lastEventReceived)
		if since > maxAge {
			ha.pr.Warnf("❌ no events received for %s - reconnecting", style.Bold(time.Since(ha.lastEventReceived).String()))

			// reconnect
			go ha.setup()

			return
		}

		ha.pr.Debugf("%s %s last event received %s ago | max age: %s | next check: %s", icons.Watchdog, icons.GreenTick.Render(), style.Bold(since.Round(time.Millisecond).String()), style.Bold(maxAge.String()), style.Bold(checkEvery.String()))
	}
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
