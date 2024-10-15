package models

import (
	"github.com/benleb/automoli-go/internal/icons"
	"github.com/benleb/automoli-go/internal/models/domain"
	"github.com/benleb/automoli-go/internal/models/service"
	"github.com/charmbracelet/log"
	mapset "github.com/deckarep/golang-set/v2"
)

const (
	AppName    = "AutoMoLi"
	AppVersion = "dev"
	AppIcon    = icons.LightOn
)

var Printer *log.Logger

// var Printer = log.NewWithOptions(os.Stdout, log.Options{
// 	// ReportTimestamp: true,
// 	ReportTimestamp: false,
// 	TimeFormat:      " " + "15:04:05",
// 	ReportCaller:    logLevel < log.InfoLevel,
// 	Level:           logLevel,
// })

// var Printer = log.NewWithOptions(os.Stdout, log.Options{
// 	// ReportTimestamp: true,
// 	ReportTimestamp: false,
// 	TimeFormat:      " " + "15:04:05",
// })

// AllowedServiceData contains the allowed keys for service_data per service and domain.
var AllowedServiceData = map[service.Service]map[domain.Domain]mapset.Set[string]{
	service.TurnOn: {
		domain.Light:  mapset.NewSet[string]("transition", "rgb_color", "rgbw_color", "rgbww_color", "color_name", "hs_color", "xy_color", "color_temp", "kelvin", "brightness", "brightness_pct", "brightness_step", "brightness_step_pct", "white", "profile", "flash", "effect"),
		domain.Scene:  mapset.NewSet[string]("transition"),
		domain.Switch: mapset.NewSet[string](),
	},
	service.TurnOff: {
		domain.Light:  mapset.NewSet[string]("transition", "flash"),
		domain.Switch: mapset.NewSet[string](),
	},
	service.Toggle: {
		domain.Light:  mapset.NewSet[string]("transition", "rgb_color", "rgbw_color", "rgbww_color", "color_name", "hs_color", "xy_color", "color_temp", "kelvin", "brightness", "brightness_pct", "brightness_step", "brightness_step_pct", "white", "profile", "flash", "effect"),
		domain.Switch: mapset.NewSet[string](),
	},
}
