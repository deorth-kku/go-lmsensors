package lmsensors

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"

	sf "github.com/mt-inside/go-lmsensors/subfeature"
)

func TestGet(t *testing.T) {
	err := Init()
	if err != nil {
		t.Error(err)
		return
	}
	defer Cleanup()
	info, err := Get()
	if err != nil {
		if !errors.Is(err, ErrSensorAny) {
			t.Error(err)
			return
		}
		var se SensorErr
		if !errors.As(err, &se) {
			t.Error(err)
			return
		}
		fmt.Println(se.Code().String(), se.SubFeature().String())
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "    ")
	err = encoder.Encode(info)
	if err != nil {
		t.Error(err)
	}
}

func TestChip(t *testing.T) {
	err := Init()
	if err != nil {
		t.Error(err)
		return
	}
	defer Cleanup()
	for no, chip := range Chips {
		fmt.Println(no, chip.Name(), chip.Path(), chip.Prefix(), chip.Bus(), chip.Addr())
	}
}

func TestFeature(t *testing.T) {
	err := Init()
	if err != nil {
		t.Error(err)
		return
	}
	defer Cleanup()
	for _, chip := range Chips {
		for _, feat := range chip.Features {
			fmt.Println(chip.Name(), chip.Path(), feat.Name(), feat.Label(), feat.Type())
			for sub, value := range feat.Values {
				fmt.Println(sub, value)
			}
			fmt.Println()
		}
	}
}

func TestGetChip(t *testing.T) {
	err := Init()
	if err != nil {
		t.Error(err)
		return
	}
	defer Cleanup()
	for _, chip := range Chips {
		nchip, err := GetChip(chip.Name())
		if err != nil {
			t.Error(err)
			return
		}
		fmt.Println(nchip.Path())
		for _, feat := range nchip.Features {
			for sub, val := range feat.Values {
				fmt.Println(sub, val)
			}
		}
		fmt.Println()
		nchip.Free()
	}

}

func TestSetValue(t *testing.T) {
	err := Init()
	if err != nil {
		t.Error(err)
		return
	}
	defer Cleanup()
	for _, chip := range Chips {
		for _, feat := range chip.Features {
			if feat.Type() != Intrusion {
				continue
			}
			err := feat.SetValue(sf.INTRUSION_ALARM, 0)
			if err != nil {
				t.Error(err)
				return
			}
			val, err := feat.GetValue(sf.INTRUSION_ALARM)
			if val != 0 {
				t.Error("set failed")
				return
			}
		}
	}
}
