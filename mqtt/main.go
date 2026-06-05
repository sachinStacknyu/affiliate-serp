package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/joho/godotenv"
)

type Config struct {
	Host       string
	Port       int
	ClientID   string
	Username   string
	Password   string
	Persistent bool
}

func loadConfig() Config {
	if err := godotenv.Load(); err != nil {
		panic("Unable to load environment variables")
	}

	port, err := strconv.Atoi(os.Getenv("MQTT_PORT"))
	if err != nil {
		port = 1883
	}

	return Config{
		Host:       os.Getenv("MQTT_HOST"),
		Port:       port,
		ClientID:   os.Getenv("MQTT_CLIENT_ID"),
		Username:   os.Getenv("MQTT_USERNAME"),
		Password:   os.Getenv("MQTT_PASSWORD"),
		Persistent: true,
	}
}

type Subscription struct {
	Topic string
	QoS   byte
}

// subscribeWithWorkerPool subscribes to all topics concurrently using a
// worker pool. The buffered jobs channel is pre-loaded with all topics so
// workers can drain it without any producer/consumer timing dependency.
func subscribeWithWorkerPool(client mqtt.Client, subs []Subscription, workerCount int) {
	jobs := make(chan Subscription, len(subs))

	var wg sync.WaitGroup

	// workers
	for i := range workerCount {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for sub := range jobs {
				token := client.Subscribe(sub.Topic, sub.QoS, nil)
				token.Wait()
				if token.Error() != nil {
					log.Printf("error:   worker-%d: subscribe to %q failed: %v", id, sub.Topic, token.Error())
					continue
				}
				fmt.Printf("info:  worker-%d: subscribed to %-30s (QoS %d)\n", id, sub.Topic, sub.QoS)
			}
		}(i)
	}

	// Push all jobs into buffered channel — never blocks because buffer == len(subs)
	for _, sub := range subs {
		jobs <- sub
	}
	close(jobs) // signal workers: no more jobs coming

	wg.Wait() // block until every worker has finished
}

func onMessageReceived(_ mqtt.Client, msg mqtt.Message) {
	fmt.Printf("msg:  topic=%-30s payload=%s\n", msg.Topic(), msg.Payload())
}

func onConnectionLost(_ mqtt.Client, err error) {
	fmt.Printf("[WARN]  connection lost: %v\n", err)
}

func ConnectToBroker(cfg Config, subs []Subscription, workerCount int) mqtt.Client {
	subReady := make(chan struct{})

	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("mqtt://%s:%d", cfg.Host, cfg.Port))
	opts.SetClientID(cfg.ClientID)
	opts.SetUsername(cfg.Username)
	opts.SetPassword(cfg.Password)

	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetKeepAlive(20 * time.Second)
	opts.SetCleanSession(!cfg.Persistent)
	opts.SetConnectTimeout(2 * time.Second)

	opts.SetWill("clients/disconnect", cfg.ClientID+" disconnected", 1, false)

	opts.SetDefaultPublishHandler(onMessageReceived)
	opts.OnConnectionLost = onConnectionLost

	opts.OnConnect = func(client mqtt.Client) {
		fmt.Println("info:   MQTT Broker Connected")

		// All subscriptions happen concurrently via worker pool
		subscribeWithWorkerPool(client, subs, workerCount)

		// Signal main goroutine that all subs are ready
		// Guard against reconnect case (channel already closed)
		select {
		case <-subReady:
			// already closed — reconnect, do nothing
		default:
			close(subReady)
		}
	}

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("error:   Connect failed: %v", token.Error())
	}

	// Block main goroutine until all subscriptions are confirmed
	<-subReady

	return client
}

func publish(client mqtt.Client, topic string, payload string, qos byte) {
	token := client.Publish(topic, qos, false, payload)
	token.Wait()
	if token.Error() != nil {
		fmt.Printf("error:   publish to %q failed: %v\n", topic, token.Error())
		return
	}
	fmt.Printf("pub:    topic=%-30s payload=%s\n", topic, payload)
}

func main() {
	fmt.Println("info:   Connecting to Broker .......")

	cfg := loadConfig()

	// Define all topics to subscribe to
	subscriptions := []Subscription{
		{Topic: "devices/#", QoS: 1},
		{Topic: "sachin/testing", QoS: 1},
		{Topic: "jaydeepBhai/sphdex", QoS: 1},
		{Topic: "meetBhai/quant", QoS: 1},
	}

	// Connect + subscribe concurrently using 3 workers
	// Main blocks here until all subscriptions are confirmed
	client := ConnectToBroker(cfg, subscriptions, 2)
	defer client.Disconnect(2500)

	// Safe to publish — all subscriptions are guaranteed active
	publish(client, "devices/test", "hello from mqtt-go-tutorial", 1)
	publish(client, "sachin/testing", "testing if publish works", 1)
	publish(client, "jaydeepBhai/shpdex", "sphdex sanity done", 1)
	publish(client, "meetBhai/quant", "Quant Works well", 1)

	// Keep alive to receive incoming messages
	fmt.Println("info:   Listening for messages (20s)...")
	time.Sleep(20 * time.Second)

	fmt.Println("info:   Done. Disconnecting.")
}
