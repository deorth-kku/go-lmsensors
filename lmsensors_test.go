package lmsensors

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
)

func TestGet(t *testing.T) {
	err := Init()
	if err != nil {
		t.Error(err)
		return
	}
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
		fmt.Println(se.Code(), se.SubFeature())
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
	Cleanup()
}
