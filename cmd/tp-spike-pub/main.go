// tp-spike-pub publishes one telemetry payload to a ThingsPanel broker —
// spike/acceptance helper (topic format: devices/telemetry, token auth).
//
//	go run ./cmd/tp-spike-pub -token water-sim-token-001 -payload '{"temperature":27.5}'
package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func main() {
	addr := flag.String("addr", "localhost:1883", "broker addr")
	token := flag.String("token", "", "device access token (MQTT username+password)")
	payload := flag.String("payload", "{}", "JSON payload")
	flag.Parse()
	if *token == "" {
		log.Fatal("-token required")
	}
	opts := mqtt.NewClientOptions().AddBroker("tcp://" + *addr).
		SetClientID(*token).SetUsername(*token).SetPassword(*token)
	c := mqtt.NewClient(opts)
	if t := c.Connect(); t.Wait() && t.Error() != nil {
		log.Fatalf("connect: %v", t.Error())
	}
	t := c.Publish("devices/telemetry", 1, false, *payload)
	t.Wait()
	log.Printf("published devices/telemetry -> %s", *payload)
	time.Sleep(time.Second)
	_ = fmt.Sprint()
}
