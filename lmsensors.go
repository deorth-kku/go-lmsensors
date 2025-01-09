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
	"fmt"
	"math"
	"strconv"
	"strings"
	"unsafe"

	"github.com/mt-inside/go-lmsensors/bus"
	sf "github.com/mt-inside/go-lmsensors/subfeature"
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

type CurrentSensor struct {
	baseSensor
}

func (s *CurrentSensor) Rendered() string {
	return strconv.FormatFloat(s.Value, 'f', 2, 64)
}

func (s *CurrentSensor) Unit() string {
	return "A"
}

func (s *CurrentSensor) Alarm() bool {
	return false
}

func (s *CurrentSensor) String() string {
	return fmt.Sprintf("%s: %s%s", s.Name, s.Rendered(), s.Unit())
}

type IntrusionSensor struct {
	Name  string
	Beep  bool
	alarm bool
}

func (s *IntrusionSensor) GetName() string {
	return s.Name
}

func (s *IntrusionSensor) Rendered() string {
	return strconv.FormatBool(s.Beep)
}

func (s *IntrusionSensor) Unit() string {
	return ""
}

func (s *IntrusionSensor) Alarm() bool {
	return s.alarm
}

func (s *IntrusionSensor) String() string {
	return fmt.Sprintf("%s: %s", s.Name, s.Rendered())
}

type UnimplementedSensor struct {
	Feature
}

func (s *UnimplementedSensor) GetName() string {
	return s.Name()
}

func (s *UnimplementedSensor) Rendered() string {
	return "0.00"
}

func (s *UnimplementedSensor) Unit() string {
	return "TODO"
}

func (s *UnimplementedSensor) Alarm() bool {
	return false
}

func (s *UnimplementedSensor) String() string {
	return fmt.Sprintf("[UNIMPLEMENTED SENSOR TYPE: %s; name: %s]", s.Type(), s.Name())
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
// Get returns an error whenever there are any sensors failed to read, while other sensors value would be available in [System].
func Get() (*System, error) {
	sys := &System{Chips: make(map[string]*Chip)}
	return sys, collectError(func(yield func(string, error) bool) {
		for _, chipptr := range chips {
			chip, err := chipptr.Chip()
			sys.Chips[chip.ID] = &chip
			if err != nil && !yield("chip="+chip.ID, err) {
				return
			}
		}
	})
}

type ChipPtr struct {
	ptr *C.sensors_chip_name
}

func (chip ChipPtr) Name() string {
	const buflen = 256
	chipNameBuf := (*C.char)(C.malloc(buflen))
	defer C.free(unsafe.Pointer(chipNameBuf))
	C.sensors_snprintf_chip_name(chipNameBuf, C.ulong(buflen), chip.ptr)
	return C.GoString(chipNameBuf)
}

func (chip ChipPtr) Path() string {
	return C.GoString(chip.ptr.path)
}

func (chip ChipPtr) Prefix() string {
	return C.GoString(chip.ptr.prefix)
}

func (chip ChipPtr) String() string {
	return chip.Name()
}

func (chip ChipPtr) Bus() string {
	return bus.Type(chip.ptr.bus._type).String() + "-" + strconv.FormatInt(int64(chip.ptr.bus.nr), 10)
}

func (chip ChipPtr) Addr() string {
	return fmt.Sprintf("0x%04x", chip.ptr.addr)
}

func (chip ChipPtr) Adapter() string {
	return C.GoString(C.sensors_get_adapter_name(&chip.ptr.bus))
}

// Chip will return an error if any of its sensors failed to read. However, the returned [Chip] struct is still vaild in such case.
func (chip ChipPtr) Chip() (Chip, error) {
	ch := Chip{
		ID:      chip.Name(),
		Type:    chip.Prefix(),
		Bus:     chip.Bus(),
		Address: chip.Addr(),
		Adapter: chip.Adapter(),
		Sensors: make(map[string]Sensor),
	}
	return ch, collectError(func(yield func(string, error) bool) {
		for _, feat := range chip.Features {
			reading, err := feat.Sensor()
			name := feat.Label()
			if err != nil && !yield("feature="+name, err) {
				return
			}
			ch.Sensors[name] = reading
		}
	})
}

// Free release the memory for this chip and make it unreadable until [Init] is called again. Call on Free twice will crash the program.
// Call on [Cleanup] after Free on any chip will also crash the program.
// There is no reason to prefer Free over [Cleanup].
func (chip ChipPtr) Free() {
	C.sensors_free_chip_name(chip.ptr)
}

func (chip ChipPtr) Features(yield func(uint32, Feature) bool) {
	i := C.int(0)
	for feature := C.sensors_get_features(chip.ptr, &i); feature != nil; feature = C.sensors_get_features(chip.ptr, &i) {
		if !yield(uint32(i), Feature{chip, feature}) {
			return
		}
	}
}

//go:linkname chips
func chips(yield func(uint32, ChipPtr) bool) {
	var chipno C.int = 0
	for cchip := C.sensors_get_detected_chips(nil, &chipno); cchip != nil; cchip = C.sensors_get_detected_chips(nil, &chipno) {
		if !yield(uint32(chipno), ChipPtr{cchip}) {
			return
		}
	}
}

type Feature struct {
	Chip ChipPtr
	ptr  *C.struct_sensors_feature
}

func (feat Feature) Name() string {
	return C.GoString(feat.ptr.name)
}

func (feat Feature) Label() string {
	clabel := C.sensors_get_label(feat.Chip.ptr, feat.ptr)
	if clabel == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(clabel))
	return C.GoString(clabel)
}

func (feat Feature) Type() LmSensorType {
	return LmSensorType(feat.ptr._type)
}

func (feat Feature) getValue(sf0 *C.struct_sensors_subfeature) (float64, error) {
	var val C.double
	cerr := C.sensors_get_value(feat.Chip.ptr, sf0.number, &val)
	if cerr != 0 {
		return 0, sensorErr{sf.SubFeature(sf0._type), cerr}
	}
	return float64(val), nil
}

func (feat Feature) GetValue(sub sf.SubFeature) (float64, error) {
	sf := C.sensors_get_subfeature(feat.Chip.ptr, feat.ptr, C.sensors_subfeature_type(sub))
	if sf == nil {
		return 0, ErrSubFeatureNotExist
	}
	return feat.getValue(sf)
}

func (feat Feature) FirstValue() (sub sf.SubFeature, val float64, err error) {
	i := C.int(0)
	sf0 := C.sensors_get_all_subfeatures(feat.Chip.ptr, feat.ptr, &i)
	if sf0 == nil {
		err = ErrSubFeatureNotExist
		return
	}
	sub = sf.SubFeature(sf0._type)
	val, err = feat.getValue(sf0)
	return
}

func (feat Feature) Values(yield func(sf.SubFeature, float64) bool) {
	var val float64
	var err error
	i := C.int(0)
	for sf0 := C.sensors_get_all_subfeatures(feat.Chip.ptr, feat.ptr, &i); sf0 != nil; sf0 = C.sensors_get_all_subfeatures(feat.Chip.ptr, feat.ptr, &i) {
		val, err = feat.getValue(sf0)
		if err != nil {
			continue
		}
		if !yield(sf.SubFeature(sf0._type), val) {
			return
		}
	}
}

func (feat Feature) Sensor() (reading Sensor, err error) {
	base := baseSensor{
		Name: feat.Label(),
	}
	_, base.Value, err = feat.FirstValue()
	if err != nil {
		return
	}
	switch feat.Type() {
	case Temperature:
		ts := &TempSensor{base, Unknown}
		reading = ts
		value, err := feat.GetValue(sf.TEMP_TYPE)
		if err == nil {
			ts.TempType = LmTempType(value)
		}
	case Voltage:
		reading = &VoltageSensor{base}
	case Fan:
		reading = &FanSensor{base}
	case Current:
		reading = &CurrentSensor{base}
	case Intrusion:
		is := &IntrusionSensor{base.Name, false, base.Value != 0}
		value, _ := feat.GetValue(sf.INTRUSION_BEEP)
		is.Beep = value != 0
	default:
		reading = &UnimplementedSensor{feat}
	}
	return
}
