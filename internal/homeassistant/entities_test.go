package homeassistant

import (
	"testing"

	"github.com/benleb/automoli-go/internal/models/domain"
)

func Test_Domain(t *testing.T) {
	tests := []struct {
		name string
		eID  EntityID
		want domain.Domain
	}{
		{
			name: "valid entity id",
			eID:  *NewEntity("binary_sensor.motion_sensor_158d00022367f9"),
			want: domain.Domain("binary_sensor"),
		},
		{
			name: "valid entity id with 'subdomain'",
			eID:  *NewEntity("light.living_room.hue"),
			want: domain.Domain("light"),
		},
		// {
		// 	name: "invalid entity id without domain",
		// 	eID:  *NewEntity("living_room"),
		// 	want: "",
		// },
		// {
		// 	name: "entity id without valid domain",
		// 	eID:  *NewEntity("basement.living_room"),
		// 	want: "",
		// },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.eID.Domain(); got != tt.want {
				t.Errorf("homeassistant.EntityID.Domain() = %v, want %v", got, tt.want)
			}
		})
	}
}
