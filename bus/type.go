package bus

//go:generate stringer -type=Type
type Type int16

const (
	ANY Type = iota - 1
	I2C
	PCI
	SPI
	VIRTUAL
	ACPI
	HID
	MDIO
	SCSI
)
