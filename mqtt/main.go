// we dont need to write the workerpool logic because in the github.com/eclipse/paho.mqtt.golang it is already dealing that thing so we just have to deal with the pub/sub and config  and handle the case for where the publisher is ready but the subscriber is not ready to receiver or vice-versa we doing that with the unbuffered channel here storing the data in the struct in the unbuffered channel here
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
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

type Subscription struct {
	Topic string
	QoS   byte
}

func loadConfig() Config {
	_ = godotenv.Load()

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

func onMessageReceived(client mqtt.Client, msg mqtt.Message) {
	log.Printf(
		"message received topic=%s payload=%s",
		msg.Topic(),
		string(msg.Payload()),
	)
}

func onConnectionLost(client mqtt.Client, err error) {
	log.Printf("connection lost: %v", err)
}

func subscribeTopics(client mqtt.Client, subs []Subscription) error {
	for _, sub := range subs {

		token := client.Subscribe(
			sub.Topic,
			sub.QoS,
			onMessageReceived,
		)

		token.Wait()

		if err := token.Error(); err != nil {
			return fmt.Errorf(
				"subscribe %s failed: %w",
				sub.Topic,
				err,
			)
		}

		log.Printf(
			"subscribed topic=%s qos=%d",
			sub.Topic,
			sub.QoS,
		)
	}

	return nil
}

func ConnectToBroker(cfg Config, subs []Subscription) (mqtt.Client, error) {

	subReady := make(chan struct{})
	opts := mqtt.NewClientOptions()

	opts.AddBroker(
		fmt.Sprintf(
			"tcp://%s:%d",
			cfg.Host,
			cfg.Port,
		),
	)

	opts.SetClientID(cfg.ClientID)
	opts.SetUsername(cfg.Username)
	opts.SetPassword(cfg.Password)

	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetCleanSession(false)

	opts.SetKeepAlive(30 * time.Second)
	opts.SetPingTimeout(10 * time.Second)
	opts.SetConnectTimeout(10 * time.Second)

	opts.SetWill(
		"clients/status",
		cfg.ClientID+" offline",
		1,
		true,
	)

	opts.OnConnectionLost = onConnectionLost

	opts.OnConnect = func(client mqtt.Client) {

		log.Println("mqtt connected")

		if err := subscribeTopics(client, subs); err != nil {
			log.Printf("subscription error: %v", err)
			return
		}

		select {
		case <-subReady:
		default:
			close(subReady)
		}
	}

	client := mqtt.NewClient(opts)

	token := client.Connect()
	token.Wait()

	if err := token.Error(); err != nil {
		return nil, err
	}

	<-subReady

	return client, nil
}

func publish(
	client mqtt.Client,
	topic string,
	payload string,
	qos byte,
) error {

	token := client.Publish(
		topic,
		qos,
		false,
		payload,
	)

	token.Wait()

	return token.Error()
}

func main() {

	cfg := loadConfig()

	subscriptions := []Subscription{
		{Topic: "devices/#", QoS: 1},
		{Topic: "sachin/testing", QoS: 1},
		{Topic: "jaydeepBhai/sphdex", QoS: 1},
		{Topic: "meetBhai/quant", QoS: 1},
	}

	client, err := ConnectToBroker(
		cfg,
		subscriptions,
	)

	if err != nil {
		log.Fatal(err)
	}

	defer client.Disconnect(1000)

	if err := publish(client, "devices/test", "hello from mqtt", 1); err != nil {
		log.Printf("publish error: %v", err)
	}
	if err := publish(client, "sachin/testing", "hello from sachin if it works or not", 1); err != nil {
		log.Printf("publish error %v", err)
	}
	if err := publish(client, "jaydeepBhai/sphdex", "hello jaydeep bhai Sphdex sanity works", 1); err != nil {
		log.Printf("publish error %v", err)
	}
	if err := publish(client, "meetBhai/quant", "quant works perfectly fine", 1); err != nil {
		log.Printf("publish error %v", err)
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	log.Println("service started")

	<-ctx.Done()

	log.Println("shutting down")
}
