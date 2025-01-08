/*
 * go-lmsensors
 *
 * Copyright (c) 2021 Matt Turner.
 */

package lmsensors

// #include <stdlib.h>
// #include <sensors/sensors.h>
// #include <sensors/error.h>
// #cgo LDFLAGS: -lsensors
import "C"

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unsafe"
)

// LmSensorType is the type of sensor (eg Temperature or Fan RPM)
//
//go:generate stringer -type=LmSensorType
type LmSensorType uint32

// https://github.com/lm-sensors/lm-sensors/blob/42f240d2a457834bcbdf4dc8b57237f97b5f5854/lib/sensors.h#L138
const (
	Voltage     LmSensorType = 0x00
	Fan         LmSensorType = 0x01
	Temperature LmSensorType = 0x02
	Power       LmSensorType = 0x03
	Energy      LmSensorType = 0x04
	Current     LmSensorType = 0x05
	Humidity    LmSensorType = 0x06

	VID       LmSensorType = 0x10
	Intrusion LmSensorType = 0x11

	BeepEnable LmSensorType = 0x18

	Unhandled LmSensorType = math.MaxInt32
)

// System contains all the chips, and all their sensors, in the system
type System struct {
	Chips map[string]*Chip
}

// Chip represents a hardware monitoring chip, which has one or more sensors attached, possibly of different types.
type Chip struct {
	ID      string
	Type    string
	Bus     string
	Address string
	Adapter string

	Sensors map[string]Sensor
}

func (c *Chip) String() string {
	return fmt.Sprintf("%s at %s:%s", c.Type, c.Bus, c.Address)
}

// Sensor represents one monitoring sensor, its type (temperature, voltage, etc), and its reading.
type Sensor interface {
	fmt.Stringer

	GetName() string
	Rendered() string
	Unit() string
	Alarm() bool
}

type baseSensor struct {
	Name  string
	Value float64
}

func (s *baseSensor) GetName() string {
	return s.Name
}

// LmTempType is the type of temperature sensor (eg Thermistor or Diode)
//
//go:generate stringer -type=LmTempType
type LmTempType int

// Not defined in a library header, but: https://github.com/lm-sensors/lm-sensors/blob/42f240d2a457834bcbdf4dc8b57237f97b5f5854/prog/sensors/chips.c#L407
const (
	Disabled     LmTempType = 0
	CPUDiode     LmTempType = 1
	Transistor   LmTempType = 2
	ThermalDiode LmTempType = 3
	Thermistor   LmTempType = 4
	AMDSI        LmTempType = 5 // ??
	IntelPECI    LmTempType = 6 // Platform Environment Control Interface

	Unknown LmTempType = math.MaxInt32
)

type TempSensor struct {
	baseSensor

	TempType LmTempType
}

func (s *TempSensor) Rendered() string {
	return strconv.FormatFloat(s.Value, 'f', 0, 64)
}

func (s *TempSensor) Unit() string {
	return "°C"
}

func (s *TempSensor) Alarm() bool {
	return false
}

func (s *TempSensor) String() string {
	var ret strings.Builder
	fmt.Fprintf(&ret, "%s: %s%s", s.Name, s.Rendered(), s.Unit())
	if s.TempType != Unknown {
		fmt.Fprintf(&ret, " (%s)", s.TempType)
	}
	return ret.String()
}

type VoltageSensor struct {
	baseSensor
}

func (s *VoltageSensor) Rendered() string {
	return strconv.FormatFloat(s.Value, 'f', 2, 64)
}

func (s *VoltageSensor) Unit() string {
	return "V"
}

func (s *VoltageSensor) Alarm() bool {
	return false
}

func (s *VoltageSensor) String() string {
	return fmt.Sprintf("%s: %s%s", s.Name, s.Rendered(), s.Unit())
}

type FanSensor struct {
	baseSensor
}

func (s *FanSensor) Rendered() string {
	return strconv.FormatFloat(s.Value, 'f', 0, 64)
}

func (s *FanSensor) Unit() string {
	return "min⁻¹"
}

func (s *FanSensor) Alarm() bool {
	return false
}

func (s *FanSensor) String() string {
	return fmt.Sprintf("%s: %s%s", s.Name, s.Rendered(), s.Unit())
}

type UnimplementedSensor struct {
	baseSensor

	sensorType LmSensorType
}

func (s *UnimplementedSensor) Rendered() string {
	return strconv.FormatFloat(s.Value, 'f', 2, 64)
}

func (s *UnimplementedSensor) Unit() string {
	return "TODO"
}

func (s *UnimplementedSensor) Alarm() bool {
	return false
}

func (s *UnimplementedSensor) String() string {
	return fmt.Sprintf("[UNIMPLEMENTED SENSOR TYPE: %s; name: %s]", s.sensorType, s.Name)
}

// Init initialises the underlying lmsensors library, eg loading its database of sensor names and curves.
func Init() error {
	cerr := C.sensors_init(nil)
	if cerr != 0 {
		return fmt.Errorf("can't configure libsensors: sensors_init() return code: %d", cerr)
	}

	return nil
}

// Cleanup release the memory allocted for underlying lmsensors library. You can't access anything after this, until the next [Init] call!
// You may call Cleanup then call [Init] again in order to reload a new config file from disk.
func Cleanup() {
	C.sensors_cleanup()
}

// Get fetches all the chips, all their sensors, and all their values.
func Get() (*System, error) {
	sys := &System{Chips: make(map[string]*Chip)}
	for chipptr := range chips {
		chip := &Chip{
			ID:      chipptr.Name(),
			Adapter: chipptr.Adapter(),
			Sensors: make(map[string]Sensor),
		}
		nameParts := strings.Split(chip.ID, "-")
		if len(nameParts) != 3 {
			return nil, fmt.Errorf("unexpected chip ID: %s", chip.ID)
		}
		chip.Type, chip.Bus, chip.Address = nameParts[0], nameParts[1], nameParts[2]
		for reading := range chipptr.Sensors {
			chip.Sensors[reading.GetName()] = reading
		}
	}
	return sys, nil
}

type ChipPtr struct {
	ptr *C.sensors_chip_name
}

func (chip ChipPtr) getValue(sf *C.struct_sensors_subfeature) (float64, error) {
	var val C.double

	cerr := C.sensors_get_value(chip.ptr, sf.number, &val)
	if cerr != 0 {
		return 0.0, fmt.Errorf("can't read sensor value: chip=%v, subfeature=%v, error=%d", chip, sf, cerr)
	}

	return float64(val), nil
}

func (chip ChipPtr) getSubfeatureValue(feature *C.struct_sensors_feature, sub C.sensors_subfeature_type) (float64, error) {
	sf := C.sensors_get_subfeature(chip.ptr, feature, sub)
	if sf == nil {
		return 0, errors.New("subfeature do not exist")
	}
	return chip.getValue(sf)
}

func (chip ChipPtr) getLabel(feature *C.struct_sensors_feature) string {
	clabel := C.sensors_get_label(chip.ptr, feature)
	if clabel == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(clabel))
	return C.GoString(clabel)
}

func (chip ChipPtr) Sensors(yield func(Sensor) bool) {
	i := C.int(0)
	for feature := C.sensors_get_features(chip.ptr, &i); feature != nil; feature = C.sensors_get_features(chip.ptr, &i) {
		sensorType := LmSensorType(feature._type)
		label := chip.getLabel(feature)

		var reading Sensor
		switch sensorType {
		case Temperature:
			value, err := chip.getSubfeatureValue(feature, C.SENSORS_SUBFEATURE_TEMP_INPUT)
			if err != nil {
				break
			}
			ts := &TempSensor{baseSensor{label, value}, Unknown}
			reading = ts
			value, err = chip.getSubfeatureValue(feature, C.SENSORS_SUBFEATURE_TEMP_TYPE)
			if err == nil {
				ts.TempType = LmTempType(value)
			}

		case Voltage:
			value, err := chip.getSubfeatureValue(feature, C.SENSORS_SUBFEATURE_IN_INPUT)
			if err == nil {
				reading = &VoltageSensor{baseSensor{label, value}}
			}

		case Fan:
			value, err := chip.getSubfeatureValue(feature, C.SENSORS_SUBFEATURE_FAN_INPUT)
			if err == nil {
				reading = &FanSensor{baseSensor{label, value}}
			}

		default:
			reading = &UnimplementedSensor{baseSensor{Name: label}, sensorType}
		}

		if reading != nil {
			if !yield(reading) {
				return
			}
		}
	}
}

func (chip ChipPtr) Name() string {
	const buflen = 256
	chipNameBuf := (*C.char)(C.malloc(buflen))
	defer C.free(unsafe.Pointer(chipNameBuf))
	C.sensors_snprintf_chip_name(chipNameBuf, C.ulong(buflen), chip.ptr)
	return C.GoString(chipNameBuf)
}

func (chip ChipPtr) Adapter() string {
	return C.GoString(C.sensors_get_adapter_name(&chip.ptr.bus))
}

// Free release the memory for this chip and make it unreadable until [Init] is called again. Call on Free twice will crash the program.
// Call on [Cleanup] after Free on any chip will also crash the program.
// There is no reason prefer Free over [Cleanup].
func (chip ChipPtr) Free() {
	C.sensors_free_chip_name(chip.ptr)
}

//go:linkname chips
func chips(yield func(ChipPtr) bool) {
	var chipno C.int = 0
	for cchip := C.sensors_get_detected_chips(nil, &chipno); cchip != nil; cchip = C.sensors_get_detected_chips(nil, &chipno) {
		if !yield(ChipPtr{cchip}) {
			return
		}
	}
}
