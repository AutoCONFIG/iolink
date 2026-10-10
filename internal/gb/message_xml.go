package gb

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strconv"
	"strings"
)

type XMLMessage struct {
	CmdType  string
	SN       string
	DeviceID string
	Status   string
	SumNum   int
	Items    []CatalogItem
}

type CatalogItem struct{ DeviceID, Name, Status, Manufacturer, Model, Owner, CivilCode, Address string }

type wireMessage struct {
	XMLName    xml.Name
	CmdType    string `xml:"CmdType"`
	SN         string `xml:"SN"`
	DeviceID   string `xml:"DeviceID"`
	Status     string `xml:"Status"`
	SumNum     string `xml:"SumNum"`
	DeviceList struct {
		Num   int        `xml:"Num,attr"`
		Items []wireItem `xml:"Item"`
	} `xml:"DeviceList"`
}

type wireItem struct {
	DeviceID     string `xml:"DeviceID"`
	Name         string `xml:"Name"`
	Status       string `xml:"Status"`
	Manufacturer string `xml:"Manufacturer"`
	Model        string `xml:"Model"`
	Owner        string `xml:"Owner"`
	CivilCode    string `xml:"CivilCode"`
	Address      string `xml:"Address"`
}

func ParseMessageXML(raw []byte, expectedDevice string) (XMLMessage, error) {
	if len(raw) == 0 || len(raw) > MaxXMLSize {
		if len(raw) > MaxXMLSize {
			return XMLMessage{}, ErrXMLTooLarge
		}
		return XMLMessage{}, ErrXMLInvalid
	}
	if err := xmlBounds(raw); err != nil {
		return XMLMessage{}, err
	}
	var wire wireMessage
	dec := xml.NewDecoder(bytes.NewReader(raw))
	dec.Strict = true
	if err := dec.Decode(&wire); err != nil {
		return XMLMessage{}, ErrXMLInvalid
	}
	if wire.XMLName.Local != "Notify" && wire.XMLName.Local != "Response" {
		return XMLMessage{}, ErrXMLInvalid
	}
	if wire.CmdType != "Keepalive" && wire.CmdType != "Catalog" {
		return XMLMessage{}, ErrXMLInvalid
	}
	if !validText(wire.CmdType, 32) || !validText(wire.SN, 64) || !validDeviceID(wire.DeviceID) || (expectedDevice != "" && wire.DeviceID != expectedDevice) {
		return XMLMessage{}, ErrXMLInvalid
	}
	msg := XMLMessage{CmdType: wire.CmdType, SN: wire.SN, DeviceID: wire.DeviceID, Status: wire.Status}
	if wire.CmdType == "Keepalive" {
		if wire.Status != "OK" && wire.Status != "ERROR" {
			return XMLMessage{}, ErrXMLInvalid
		}
		return msg, nil
	}
	if wire.SumNum == "" || len(wire.DeviceList.Items) > MaxCatalogItems {
		return XMLMessage{}, ErrXMLInvalid
	}
	sum, err := strconv.Atoi(wire.SumNum)
	if err != nil || sum < 0 || sum > MaxCatalogItems || sum != len(wire.DeviceList.Items) || wire.DeviceList.Num != sum {
		return XMLMessage{}, ErrXMLInvalid
	}
	msg.SumNum = sum
	msg.Items = make([]CatalogItem, 0, sum)
	for _, item := range wire.DeviceList.Items {
		if !validDeviceID(item.DeviceID) || !validText(item.Name, 256) || (item.Status != "ON" && item.Status != "OFF") {
			return XMLMessage{}, ErrXMLInvalid
		}
		msg.Items = append(msg.Items, CatalogItem{DeviceID: item.DeviceID, Name: item.Name, Status: item.Status, Manufacturer: item.Manufacturer, Model: item.Model, Owner: item.Owner, CivilCode: item.CivilCode, Address: item.Address})
	}
	return msg, nil
}

func xmlBounds(raw []byte) error {
	upper := strings.ToUpper(string(raw))
	for _, marker := range []string{"<!DOCTYPE", "<!ENTITY", "<!["} {
		if strings.Contains(upper, marker) {
			return ErrXMLInvalid
		}
	}
	dec := xml.NewDecoder(bytes.NewReader(raw))
	dec.Strict = true
	depth, textBytes := 0, 0
	for {
		tok, err := dec.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return ErrXMLInvalid
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if depth > MaxXMLDepth {
				return ErrXMLDepth
			}
		case xml.EndElement:
			depth--
			if depth < 0 {
				return ErrXMLInvalid
			}
		case xml.CharData:
			textBytes += len(t)
			if textBytes > MaxXMLSize {
				return ErrXMLTooLarge
			}
		case xml.Directive:
			return ErrXMLInvalid
		}
	}
	if depth != 0 {
		return ErrXMLInvalid
	}
	return nil
}
