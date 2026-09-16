// mqtt-sim simulates a water-quality monitoring terminal for development
// and M1 acceptance: connects to the access broker and publishes
// iolink/up/{device}/properties on an interval.
//
// Usage:
//
//	go run ./cmd/mqtt-sim -device dev-001 -secret s3cret -interval 5s
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func main() {
	addr := flag.String("addr", "localhost:1883", "mqtt broker addr")
	device := flag.String("device", "dev-001", "device_no (MQTT username)")
	secret := flag.String("secret", "", "device secret (MQTT password)")
	interval := flag.Duration("interval", 5*time.Second, "report interval")
	flag.Parse()
	if *secret == "" {
		log.Fatal("-secret required (device triple auth)")
	}

	opts := mqtt.NewClientOptions().
		AddBroker(fmt.Sprintf("tcp://%s", *addr)).
		SetClientID(*device).
		SetUsername(*device).
		SetPassword(*secret).
		SetCleanSession(true)
	client := mqtt.NewClient(opts)
	if tok := client.Connect(); tok.Wait() && tok.Error() != nil {
		log.Fatalf("connect: %v", tok.Error())
	}
	log.Printf("connected as %s, reporting every %s", *device, *interval)

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for {
		payload, _ := json.Marshal(map[string]float64{
			"temperature":      26 + r.Float64()*3,    // 26-29 ℃
			"dissolved_oxygen": 5 + r.Float64()*3,     // 5-8 mg/L
			"ph":               7.5 + r.Float64()*1.0, // 7.5-8.5
			"turbidity":        10 + r.Float64()*10,   // 10-20 NTU
			"salinity":         28 + r.Float64()*2,    // 28-30 ppt
			"battery":          80 + r.Float64()*20,   // 80-100 %
		})
		topic := fmt.Sprintf("iolink/up/%s/properties", *device)
		if tok := client.Publish(topic, 1, false, payload); tok.Wait() && tok.Error() != nil {
			log.Printf("publish: %v", tok.Error())
		} else {
			log.Printf("published %s -> %s", topic, payload)
		}
		time.Sleep(*interval)
	}
}
