package homeassistant

import (
	"fmt"
	"reflect"

	"github.com/benleb/automoli-go/internal/models"
	"github.com/mitchellh/mapstructure"
)

func StringToEntityIDHookFunc() mapstructure.DecodeHookFunc { //nolint:ireturn
	return func(f reflect.Type, targetType reflect.Type, data any) (any, error) {
		if f.Kind() != reflect.String {
			return data, nil
		}

		if targetType != reflect.TypeOf(EntityID{}) {
			return data, nil
		}

		if rawEntityID, ok := data.(string); ok {
			return NewEntityID(rawEntityID)
		}

		return nil, models.InvalidEntityIDErr(fmt.Sprint(data))
	}
}
