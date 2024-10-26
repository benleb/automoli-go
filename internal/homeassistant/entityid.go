package homeassistant

import (
	"fmt"
	"strings"

	"github.com/benleb/automoli-go/internal/models"
	"github.com/benleb/automoli-go/internal/models/domain"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

type EntityID struct {
	ID string `json:"entity_id" mapstructure:"entity_id"`
}

// NewEntityID creates a new entity id.
func NewEntityID(rawEntityID string) (*EntityID, error) {
	if rawEntityID == "" {
		return nil, models.EmptyEntityIDErr()
	}

	entityDomain, entity, found := strings.Cut(rawEntityID, ".")
	if !found || entityDomain == "" || entity == "" {
		log.Debugf("invalid entity id: %s | before: %s | after: %s | found: %t", rawEntityID, entityDomain, entity, found)

		return nil, models.InvalidEntityIDErr(rawEntityID)
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

	dom, entityName, _ := strings.Cut(eID.ID, ".")

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

// FmtShortWithStyles returns the entity id as pretty formatted string without domain 💄.
func (eID *EntityID) FmtShortWithStyles(dotStyle lipgloss.Style, nameStyle lipgloss.Style) string {
	_, entityName, _ := strings.Cut(eID.ID, ".")

	return dotStyle.Render("…") + nameStyle.Render(entityName)
}

// Domain returns the domain part of the entity id.
func (eID *EntityID) Domain() domain.Domain {
	rawDomain, _, _ := strings.Cut(eID.ID, ".")

	dom := domain.Domain(rawDomain)
	if !dom.IsValid() {
		log.Errorf("invalid domain: %s", rawDomain)

		return ""
	}

	return dom
}

// EntityName returns the non-domain part (after the dot) of the entity id.
func (eID *EntityID) EntityName() string {
	_, entityName, _ := strings.Cut(eID.ID, ".")

	return entityName
}

func (eID *EntityID) UnmarshalText(text []byte) error {
	entityID, err := NewEntityID(string(text))
	if err != nil {
		log.Errorf("EntityName invalid entity id: %s", text)

		return fmt.Errorf("%w: %s", models.ErrInvalidEntityID, text)
	}

	*eID = *entityID

	return nil
}

func (eID *EntityID) MarshalText() ([]byte, error) {
	return []byte(eID.ID), nil
}
