package lmsensors

import (
	"encoding/json"
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
	fmt.Println(err)
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
	for no, chip := range chips {
		fmt.Println(no, chip.Name(), chip.Path(), chip.Prefix(), chip.Bus(), chip.Addr())
	}
}

func TestFeature(t *testing.T) {
	err := Init()
	if err != nil {
		t.Error(err)
		return
	}
	for _, chip := range chips {
		for _, feat := range chip.Features {
			fmt.Println(chip.Name(), chip.Path(), feat.Name(), feat.Label(), feat.Type())
			for sub, value := range feat.Values {
				fmt.Println(sub, value)
			}
			fmt.Println()
		}
	}
}
