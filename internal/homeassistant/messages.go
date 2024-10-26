package homeassistant

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/benleb/automoli-go/internal/icons"
	"github.com/benleb/automoli-go/internal/models/domain"
	"github.com/benleb/automoli-go/internal/models/service"
	"github.com/benleb/automoli-go/internal/style"
	"github.com/charmbracelet/lipgloss"
)

// Message is the interface for all messages sent to or received from Home Assistant.
type Message interface {
	// SetID sets the message ID and returns it.
	SetID(id int64) int64
	// GetID returns the message ID.
	GetID() int64

	// String returns a string representation of the message.
	String() string
}

// baseMessage is the base struct for all messages sent to or received from Home Assistant.
type baseMessage struct {
	ID   int64  `json:"id,omitempty" mapstructure:"id,omitempty"`
	Type string `json:"type"         mapstructure:"type"`
}

func (m *baseMessage) SetID(id int64) int64 {
	m.ID = id

	return m.ID
}

func (m *baseMessage) GetID() int64 {
	return m.ID
}

func (m *baseMessage) framelessString() string {
	out := strings.Builder{}
	out.WriteString(style.Gray(6).Render("#"))
	out.WriteString(strconv.FormatInt(m.ID, 10))
	out.WriteString(style.ColorizeHABlue("|"))

	return out.String()
}

func (m *baseMessage) framelessStringWithType() string {
	out := strings.Builder{}
	out.WriteString(m.framelessString())
	out.WriteString(style.Gray(8).Render(m.Type))

	return out.String()
}

func (m *baseMessage) String() string {
	return style.HABlueFrame(m.framelessStringWithType())
}

type VersionMsg struct {
	baseMessage `mapstructure:",squash"`
	HaVersion   string `json:"ha_version"`
}

type MessageMsg struct {
	baseMessage `mapstructure:",squash"`
	Message     string `json:"message"`
}

type AuthMsg struct {
	baseMessage `mapstructure:",squash"`
	AccessToken string `json:"access_token"`
}

func NewAuthMsg(token string) AuthMsg {
	return AuthMsg{
		baseMessage: baseMessage{Type: "auth"},
		AccessToken: token,
	}
}

type CallServiceMsg struct {
	baseMessage `mapstructure:",squash"`

	Service     service.Service `json:"service"`
	Domain      domain.Domain   `json:"domain"`
	ServiceData interface{}     `json:"service_data,omitempty"`
	Target      Target          `json:"target,omitempty"`
}

func (m *CallServiceMsg) String() string {
	serviceData := make([]string, 0)
	if sd, ok := m.ServiceData.(map[string]interface{}); ok {
		for k, v := range sd {
			serviceData = append(serviceData, style.Gray(8).Render(k)+style.ColorizeHABlue(":")+fmt.Sprintf("%v", v))
		}
	}
	fmtServiceData := strings.Join(serviceData, style.ColorizeHABlue("|"))

	out := strings.Builder{}

	out.WriteString(m.baseMessage.framelessStringWithType())
	out.WriteString(style.ColorizeHABlue("|"))
	out.WriteString(style.Gray(6).Render("…") + lipgloss.NewStyle().Foreground(lipgloss.Color("#ddd")).Italic(true).Render(string(m.Service)))
	out.WriteString(style.ColorizeHABlue(" → "))
	out.WriteString(fmt.Sprint(m.Target.EntityID))
	// out.WriteString(m.Target.EntityID.FmtString())

	if len(serviceData) > 0 {
		out.WriteString(" " + style.HABlueFrame(fmtServiceData))
	}

	return style.HABlueFrame(out.String())
}

type Target struct {
	EntityID EntityID `json:"entity_id"`
}

func NewCallServiceMsg(service service.Service, serviceData map[string]interface{}, target EntityID) *CallServiceMsg {
	serviceCallMsg := &CallServiceMsg{
		baseMessage: baseMessage{
			Type: "call_service",
		},
		Service: service,
		Domain:  target.Domain(),
		Target: Target{
			EntityID: target,
		},
	}

	if len(serviceData) > 0 {
		serviceCallMsg.ServiceData = serviceData
	}

	return serviceCallMsg
}

type SubscribeMsg struct {
	baseMessage `mapstructure:",squash"`
	EventType   EventType `json:"event_type"`
}

func (m *SubscribeMsg) String() string {
	out := strings.Builder{}

	out.WriteString(m.baseMessage.framelessStringWithType())
	out.WriteString(style.ColorizeHABlue(" → "))
	out.WriteString(style.Bold(string(m.EventType)))

	return style.HABlueFrame(out.String())
}

func NewSubscribeMsg(eventType EventType) *SubscribeMsg {
	return &SubscribeMsg{
		baseMessage: baseMessage{
			Type: "subscribe_events",
		},
		EventType: eventType,
	}
}

type EventMsg struct {
	baseMessage `mapstructure:",squash"`
	Event       *event `json:"event"           mapstructure:"event"`
}

type ResultMsg struct {
	baseMessage `mapstructure:",squash"`
	Success     bool        `json:"success"         mapstructure:"success"`
	Result      any         `json:"result"          mapstructure:"result"`
	Error       ErrorResult `json:"error"           mapstructure:"error,omitempty"`
}

func (m *ResultMsg) String() string {
	out := strings.Builder{}

	out.WriteString(m.baseMessage.framelessString())

	var icon string
	if m.Success {
		icon = icons.GreenTick.String()
		out.WriteString("success")
	} else {
		icon = icons.RedCross.String()
		out.WriteString("fail")
	}

	resultsMap, ok := m.Result.([]interface{})
	if ok && len(resultsMap) > 0 && len(resultsMap) < 10 {
		out.WriteString(style.ColorizeHABlue(" → "))
		out.WriteString(fmt.Sprintf("%+v", m.Result))
	}

	return " " + icon + " " + style.HABlueFrame(out.String())
}

type ErrorResult struct {
	Code    string `json:"code"    mapstructure:"code"`
	Message string `json:"message" mapstructure:"message"`
}
