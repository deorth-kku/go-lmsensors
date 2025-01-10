/*
 * go-lmsensors
 *
 * Copyright (c) 2021 Matt Turner.
 */

package lmsensors

// #include <stdlib.h>
// #include <sensors/sensors.h>
// #cgo LDFLAGS: -lsensors
import "C"

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
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
		for _, chipptr := range Chips {
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
	return strings.Join([]string{chip.Prefix(), chip.Bus(), chip.Addr()}, "-")
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
	bus := strings.ToLower(bus.Type(chip.ptr.bus._type).String())
	if chip.hasNR() {
		bus += "-" + strconv.FormatInt(int64(chip.ptr.bus.nr), 10)
	}
	return bus
}

func (chip ChipPtr) addrfmt() string {
	switch chip.ptr.bus._type {
	case C.SENSORS_BUS_TYPE_ISA, C.SENSORS_BUS_TYPE_PCI:
		return "%04x"
	case C.SENSORS_BUS_TYPE_I2C:
		return "%02x"
	default:
		return "%x"
	}
}

func (chip ChipPtr) Addr() string {
	return fmt.Sprintf(chip.addrfmt(), chip.ptr.addr)
}

func (chip ChipPtr) Adapter() string {
	return C.GoString(C.sensors_get_adapter_name(&chip.ptr.bus))
}

// Chip will return an error if any of its sensors failed to read. However, the returned [Chip] struct is still valid in such case.
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
			ch.Sensors[name] = reading
			if err != nil && !yield("feature="+name, err) {
				return
			}
		}
	})
}

// Feature is an iterator for range over all features for the chip, and it's the only way to create a valid [Feature] object.
func (chip ChipPtr) Features(yield func(uint32, Feature) bool) {
	i := C.int(0)
	for feature := C.sensors_get_features(chip.ptr, &i); feature != nil; feature = C.sensors_get_features(chip.ptr, &i) {
		if !yield(uint32(i), Feature{chip, feature}) {
			return
		}
	}
}

// Chips is an iterator for range over all chips. As they are detected in [Init].
func Chips(yield func(uint32, ChipPtr) bool) {
	chipno := C.int(0)
	for cchip := C.sensors_get_detected_chips(nil, &chipno); cchip != nil; cchip = C.sensors_get_detected_chips(nil, &chipno) {
		if !yield(uint32(chipno), ChipPtr{cchip}) {
			return
		}
	}
}

func (chip ChipPtr) hasWildcards() bool {
	return chip.ptr.prefix == nil ||
		chip.ptr.bus._type == C.SENSORS_BUS_TYPE_ANY ||
		chip.ptr.bus.nr == C.SENSORS_BUS_NR_ANY ||
		chip.ptr.addr == C.SENSORS_CHIP_NAME_ADDR_ANY
}

func (chip ChipPtr) hasNR() bool {
	switch chip.ptr.bus._type {
	case C.SENSORS_BUS_TYPE_I2C, C.SENSORS_BUS_TYPE_SPI, C.SENSORS_BUS_TYPE_HID, C.SENSORS_BUS_TYPE_SCSI:
		return true
	default:
		return false
	}
}

const hwmon_dir = "/sys/class/hwmon"

func (chip ChipPtr) searchSetPath() {
	entries, err := os.ReadDir(hwmon_dir)
	if err != nil {
		return
	}
	prefix := chip.Prefix()
	for _, e := range entries {
		p, err := os.ReadFile(filepath.Join(hwmon_dir, e.Name(), "name"))
		if err != nil || len(p) == 0 {
			continue
		}
		p = p[:len(p)-1]
		if string(p) == prefix {
			chip.ptr.path = C.CString(filepath.Join(hwmon_dir, e.Name()))
			break
		}
	}
}

// GetChip create a [ChipPtr] from a name, as it was definded in https://github.com/lm-sensors/lm-sensors/blob/42f240d2a457834bcbdf4dc8b57237f97b5f5854/lib/data.c#L62
// However, I prohibit wildcards here because none of the methods allow wildcards.
// You need to call [ChipPtr.Free] only if ChipPtr is created from this function.
func GetChip(name string) (ChipPtr, error) {
	ch := ChipPtr{new(C.sensors_chip_name)}
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	cerr := C.sensors_parse_chip_name(cname, ch.ptr)
	if cerr != 0 {
		ch.Free()
		return ChipPtr{}, SensorErrCode(cerr)
	}
	if !ch.hasNR() {
		ch.ptr.bus.nr = 0
	}
	if ch.hasWildcards() {
		ch.Free()
		return ChipPtr{}, ErrSensorWildcards
	}
	ch.searchSetPath()
	if ch.ptr.path == nil {
		ch.Free()
		return ChipPtr{}, ErrSensorChipName
	}
	return ch, nil
}

// Free release the C memory allocated for this [ChipPtr].
// Only call Free if a [ChipPtr] is created from [GetChip].
func (chip ChipPtr) Free() {
	if chip.ptr.path != nil {
		C.free(unsafe.Pointer(chip.ptr.path))
	}
	if chip.ptr.prefix != nil {
		C.free(unsafe.Pointer(chip.ptr.prefix))
	}
}

type Feature struct {
	Chip ChipPtr
	ptr  *C.struct_sensors_feature
}

// Name return the original name of a sensor.
func (feat Feature) Name() string {
	return C.GoString(feat.ptr.name)
}

// Label return the labed of a sensor which is set by config file.
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

// SubFeatures is an iterator for range over all subfeatures without reading it's value.
func (feat Feature) SubFeatures(yield func(sf.SubFeature) bool) {
	i := C.int(0)
	for sf0 := C.sensors_get_all_subfeatures(feat.Chip.ptr, feat.ptr, &i); sf0 != nil; sf0 = C.sensors_get_all_subfeatures(feat.Chip.ptr, feat.ptr, &i) {
		if !yield(sf.SubFeature(sf0._type)) {
			return
		}
	}
}

// Values is an iterator for range over all subfeatures and it's value.
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

// Sensor read sensor data into a [Sensor] interface.
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
