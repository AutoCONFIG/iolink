// Package wire defines the MQTT payload contract between device firmware
// and the access module. Hardware teams code against these shapes; the
// authoritative human-readable spec lives in docs/mqtt-spec.md.
package wire

// Report is the property-uplink payload (topic: iolink/up/{device_no}/properties).
type Report struct {
	Temperature *float64 `json:"temperature,omitempty"`      // ℃
	DO          *float64 `json:"dissolved_oxygen,omitempty"` // mg/L
	PH          *float64 `json:"ph,omitempty"`               // pH
	Turbidity   *float64 `json:"turbidity,omitempty"`        // NTU
	Salinity    *float64 `json:"salinity,omitempty"`         // ppt
	Battery     *float64 `json:"battery,omitempty"`          // 0-100 %
	Signal      *int     `json:"signal,omitempty"`           // RSSI dBm, -120~0
}

// DeviceStatus is the device health block, reported alongside or separately.
type DeviceStatus struct {
	Battery *float64 `json:"battery,omitempty"`
	Signal  *int     `json:"signal,omitempty"` // RSSI dBm
}

// Command is the downlink envelope (topic: iolink/down/{device_no}/cmd).
// Reserved for phase 2 (remote control); firmware may ignore it for now.
type Command struct {
	ID     string            `json:"id"`   // command id, echoed in ack
	Name   string            `json:"name"` // e.g. "set_interval"
	Params map[string]string `json:"params,omitempty"`
}

// Ack is the device response to a command.
type Ack struct {
	ID    string `json:"id"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}
