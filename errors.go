package lmsensors

// #include <sensors/error.h>
// #cgo LDFLAGS: -lsensors
import "C"
import (
	"fmt"
	"iter"
	"math"
	"strings"

	sf "github.com/mt-inside/go-lmsensors/subfeature"
)

type SensorErr interface {
	error
	SubFeature() sf.SubFeature
	Code() SensorErrCode
}

var (
	ErrSubFeatureNotExist           = sensorErr{}
	_                     SensorErr = ErrSubFeatureNotExist
)

// sensorErr is only allowed to create within the package, and cannot be modified.
// To read its value, you can cast it into [SensorErr]
type sensorErr struct {
	sub  sf.SubFeature
	cerr C.int
}

func (s sensorErr) SubFeature() sf.SubFeature {
	return s.sub
}

func (s sensorErr) Code() SensorErrCode {
	return SensorErrCode(s.cerr)
}

func (s sensorErr) Error() string {
	return fmt.Sprintf("can't read sensor value: subfeature=%s, error=%d", s.sub, s.cerr)
}

func (s sensorErr) Is(err error) bool {
	switch code := err.(type) {
	case SensorErrCode:
		if code == ErrSensorAny {
			return true
		}
		return code == SensorErrCode(s.cerr)
	case sf.SubFeature:
		return code == s.sub
	case sensorErr:
		return s == code
	default:
		return false
	}
}

//go:generate stringer -type=SensorErrCode
type SensorErrCode int32

func (s SensorErrCode) Error() string {
	return s.String()
}

const (
	ErrSensorWildcards SensorErrCode = -C.SENSORS_ERR_WILDCARDS // Wildcard found in chip name
	ErrSensorNoEntry   SensorErrCode = -C.SENSORS_ERR_NO_ENTRY  // No such subfeature known
	ErrSensorAccessR   SensorErrCode = -C.SENSORS_ERR_ACCESS_R  // Can't read
	ErrSensorKernel    SensorErrCode = -C.SENSORS_ERR_KERNEL    // Kernel interface error
	ErrSensorDivZero   SensorErrCode = -C.SENSORS_ERR_DIV_ZERO  // Divide by zero
	ErrSensorChipName  SensorErrCode = -C.SENSORS_ERR_CHIP_NAME // Can't parse chip name
	ErrSensorBusName   SensorErrCode = -C.SENSORS_ERR_BUS_NAME  // Can't parse bus name
	ErrSensorParse     SensorErrCode = -C.SENSORS_ERR_PARSE     // General parse error
	ErrSensorAccessW   SensorErrCode = -C.SENSORS_ERR_ACCESS_W  // Can't write
	ErrSensorIO        SensorErrCode = -C.SENSORS_ERR_IO        // I/O error
	ErrSensorRecursion SensorErrCode = -C.SENSORS_ERR_RECURSION // Evaluation recurses too deep

	ErrSensorAny SensorErrCode = math.MaxInt32 // A special case for [sensorErr.Is] to always match
)

type wrapError struct {
	msg string
	err error
}

func (w wrapError) Error() string {
	return fmt.Sprintf("%s: %s", w.msg, w.err)
}

func (w wrapError) Unwrap() error {
	return w.err
}

type wrapErrors []wrapError

func (w wrapErrors) Error() string {
	if len(w) == 0 {
		return ""
	}
	args := make([]any, 0, len(w)*2)
	for _, e := range w {
		args = append(args, e.msg, e.err)
	}
	fmtstr := "%s: %s" + strings.Repeat(", %s: %s", len(w)-1)
	return fmt.Sprintf(fmtstr, args...)
}

func (w wrapErrors) Unwrap() []error {
	errs := make([]error, len(w))
	for i := range errs {
		errs[i] = w[i].err
	}
	return errs
}

func collectError(it iter.Seq2[string, error]) error {
	var w wrapErrors
	for k, v := range it {
		w = append(w, wrapError{k, v})
	}
	if len(w) == 0 {
		return nil
	}
	return w
}
