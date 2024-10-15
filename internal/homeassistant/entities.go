package homeassistant

import (
	"errors"
	"fmt"
	"strings"

	"github.com/benleb/automoli-go/internal/models/domain"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

type EntityID struct {
	// ha *HomeAssistant `mapstructure:"-"`
	ID string `json:"entity_id" mapstructure:"entity_id"`
}

func (eID *EntityID) UnmarshalText(text []byte) error {
	entityID, err := NewEntityID(string(text))
	if err != nil {
		log.Errorf("EntityName invalid entity id: %s", text)

		return fmt.Errorf("%w: %s", errInvalidEntityID, text)
	}

	*eID = *entityID

	return nil
}

func (eID *EntityID) MarshalText() ([]byte, error) {
	return []byte(eID.ID), nil
}

// NewEntity returns a new entity.
func NewEntity(rawEntityID string) *EntityID {
	entityID, err := NewEntityID(rawEntityID)
	if err != nil {
		log.Errorf("NewEntityID invalid entity id: %s", rawEntityID)

		return nil
	}

	return entityID
}

var (
	errEmptyEntityID   = errors.New("empty entity id")
	errInvalidEntityID = errors.New("invalid entity id")
	errInvalidDomain   = errors.New("invalid domain")
)

func InvalidEntityID(rawEntityID string) error {
	return fmt.Errorf("%w: %s", errInvalidEntityID, rawEntityID)
}

func EmptyEntityID() error {
	return fmt.Errorf("%w", errEmptyEntityID)
}

func InvalidDomain(domain string) error {
	return fmt.Errorf("%w: %s", errInvalidDomain, domain)
}

func NewEntityID(rawEntityID string) (*EntityID, error) {
	if rawEntityID == "" {
		return nil, EmptyEntityID()
	}

	parts := strings.SplitN(rawEntityID, ".", 2)
	if len(parts) != 2 {
		return nil, InvalidEntityID(rawEntityID)
	}

	if dom := domain.Domain(parts[0]); !dom.IsValid() {
		return nil, InvalidDomain(parts[0])
	}

	return &EntityID{ID: rawEntityID}, nil
}

// String returns the entity id as string.
func (eID *EntityID) String() string { return eID.ID }

// FmtString returns the entity id as pretty formatted string 💄.
func (eID *EntityID) FmtString() string {
	if eID == nil || eID.ID == "" {
		log.Errorf("cannot format empty entity id: %#v", eID)

		return ""
	}

	parts := strings.SplitN(eID.ID, ".", 2)

	if len(parts) != 2 {
		log.Errorf("invalid entity id: %#v", eID)

		return ""
	}

	dom := parts[0]
	entityName := parts[1]

	brightStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#ddd")).Bold(true)
	darkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#999")).Bold(false)

	return darkStyle.Render(dom) + "." + brightStyle.Render(entityName)
}

// FmtShort returns the entity id as pretty formatted string without domain 💄.
func (eID *EntityID) FmtShort() string {
	return eID.FmtShortWithStyles(
		lipgloss.NewStyle().Foreground(lipgloss.Color("#999")).Bold(true),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#666")).Bold(false),
	)
}

func (eID *EntityID) FmtShortWithStyles(dotStyle lipgloss.Style, nameStyle lipgloss.Style) string {
	parts := strings.SplitN(eID.ID, ".", 2)
	if len(parts) != 2 {
		log.Errorf("FmtShortWithStyles invalid entity id: %s", eID.ID)

		return ""
	}

	entityName := parts[1]

	return dotStyle.Render("…") + nameStyle.Render(entityName)
}

// Domain returns the domain part of the entity id.
func (eID *EntityID) Domain() domain.Domain {
	parts := strings.SplitN(eID.ID, ".", 2)
	if len(parts) != 2 {
		log.Errorf("Domain invalid entity id: %s", eID.ID)

		return ""
	}

	dom := domain.Domain(parts[0])
	if !dom.IsValid() {
		log.Errorf("invalid domain: %s", parts[0])

		return ""
	}

	return dom
}

// EntityName returns the non-domain part (after the dot) of the entity id.
func (eID *EntityID) EntityName() string {
	parts := strings.SplitN(eID.ID, ".", 2)
	if len(parts) != 2 {
		log.Errorf("EntityName invalid entity id: %s", eID.ID)

		return ""
	}

	return parts[1]
}
