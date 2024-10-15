package domain

import mapset "github.com/deckarep/golang-set/v2"

const (
	BinarySensor Domain = "binary_sensor"
	Light        Domain = "light"
	Scene        Domain = "scene"
	Sensor       Domain = "sensor"
	Switch       Domain = "switch"
)

var ValidDomains = mapset.NewSet[Domain](BinarySensor, Light, Scene, Sensor, Switch)

type Domain string

func (d Domain) String() string { return string(d) }
func (d Domain) IsValid() bool  { return ValidDomains.Contains(d) }

// func IsValid(dom Domain) bool { return ValidDomains.Contains(dom) }
