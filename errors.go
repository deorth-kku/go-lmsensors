package lmsensors

import (
	"C"
	"fmt"

	sf "github.com/mt-inside/go-lmsensors/subfeature"
)
import (
	"iter"
	"strings"
)

var ErrSubFeatureNotExist = sensorErr{}

type sensorErr struct {
	sub  sf.SubFeature
	cerr C.int
}

func (s sensorErr) Error() string {
	return fmt.Sprintf("can't read sensor value: subfeature=%s, error=%d", s.sub, s.cerr)
}

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
