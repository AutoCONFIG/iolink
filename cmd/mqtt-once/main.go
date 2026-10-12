// mqtt-once publishes a single one-shot payload — dev/acceptance helper.
//
//	go run ./cmd/mqtt-once -device dev-001 -username iolink-dev-abc12345-abcdef -secret dev-secret-001 -payload '{"dissolved_oxygen":3.6}'
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
	device := flag.String("device", "dev-001", "device_no (topic identity)")
	username := flag.String("username", "", "issued MQTT username (iolink-<device_no>-xxxxxx)")
	secret := flag.String("secret", "", "device secret (MQTT password)")
	kind := flag.String("kind", "properties", "uplink kind: properties|ack")
	payload := flag.String("payload", "{}", "JSON payload")
	flag.Parse()
	if *secret == "" || *username == "" {
		log.Fatal("-username and -secret required (issued credential pair)")
	}
	var pretty map[string]any
	_ = json.Unmarshal([]byte(*payload), &pretty)

	opts := mqtt.NewClientOptions().AddBroker("tcp://" + *addr).
		SetClientID(*device).SetUsername(*username).SetPassword(*secret)
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
