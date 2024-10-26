package domain

import (
	mapset "github.com/deckarep/golang-set/v2"
)

const (
	BinarySensor Domain = "binary_sensor"
	InputBoolean Domain = "input_boolean"
	Light        Domain = "light"
	Scene        Domain = "scene"
	Sensor       Domain = "sensor"
	Switch       Domain = "switch"
)

var validDomains = mapset.NewSet(BinarySensor, InputBoolean, Light, Scene, Sensor, Switch)

type Domain string

func (d Domain) String() string { return string(d) }
func (d Domain) IsValid() bool  { return validDomains.Contains(d) }
