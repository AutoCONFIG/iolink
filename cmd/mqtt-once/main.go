// mqtt-once publishes a single one-shot payload — dev/acceptance helper.
//
//	go run ./cmd/mqtt-once -device dev-001 -secret dev-secret-001 -payload '{"dissolved_oxygen":3.6}'
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func main() {
	addr := flag.String("addr", "localhost:1883", "mqtt broker addr")
	device := flag.String("device", "dev-001", "device_no")
	secret := flag.String("secret", "", "device secret")
	kind := flag.String("kind", "properties", "uplink kind: properties|ack")
	payload := flag.String("payload", "{}", "JSON payload")
	flag.Parse()
	if *secret == "" {
		log.Fatal("-secret required")
	}
	var pretty map[string]any
	_ = json.Unmarshal([]byte(*payload), &pretty)

	opts := mqtt.NewClientOptions().AddBroker("tcp://" + *addr).
		SetClientID(*device).SetUsername(*device).SetPassword(*secret)
	client := mqtt.NewClient(opts)
	if tok := client.Connect(); tok.Wait() && tok.Error() != nil {
		log.Fatalf("connect: %v", tok.Error())
	}
	topic := fmt.Sprintf("iolink/up/%s/%s", *device, *kind)
	if tok := client.Publish(topic, 1, false, *payload); tok.Wait() && tok.Error() != nil {
		log.Fatalf("publish: %v", tok.Error())
	}
	log.Printf("published %s -> %s", topic, *payload)
	time.Sleep(time.Second)
}
