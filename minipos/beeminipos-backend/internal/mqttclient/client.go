package mqttclient

import (
	"context"
	"fmt"
	"log"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"fiscalisation/beeminipos-backend/internal/config"
)

// Start connects to EMQX and subscribes to configured topics.
func Start(ctx context.Context, cfg config.Config, logger *log.Logger) (func(), error) {
	if cfg.EMQXBroker == "" || cfg.EMQXToken == "" || len(cfg.EMQXSubTopics) == 0 {
		logger.Printf("mqtt disabled: broker/token/topics are not fully configured")
		return nil, nil
	}

	handler := func(_ mqtt.Client, msg mqtt.Message) {
		logger.Printf("mqtt message topic=%s bytes=%d", msg.Topic(), len(msg.Payload()))
	}

	subscribe := func(client mqtt.Client) error {
		for _, topic := range cfg.EMQXSubTopics {
			token := client.Subscribe(topic, 1, handler)
			if ok := token.WaitTimeout(10 * time.Second); !ok {
				return fmt.Errorf("subscribe timeout for topic %s", topic)
			}
			if err := token.Error(); err != nil {
				return fmt.Errorf("subscribe topic %s: %w", topic, err)
			}
		}
		logger.Printf("mqtt subscribed to topics: %v", cfg.EMQXSubTopics)
		return nil
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(cfg.EMQXBroker)
	opts.SetClientID(cfg.EMQXClientID)
	opts.SetUsername(cfg.EMQXUsername)
	opts.SetPassword(cfg.EMQXToken)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(5 * time.Second)
	opts.SetOnConnectHandler(func(client mqtt.Client) {
		if err := subscribe(client); err != nil {
			logger.Printf("mqtt subscribe error: %v", err)
		}
	})
	opts.SetConnectionLostHandler(func(_ mqtt.Client, err error) {
		logger.Printf("mqtt connection lost: %v", err)
	})

	client := mqtt.NewClient(opts)
	connectToken := client.Connect()
	if ok := connectToken.WaitTimeout(15 * time.Second); !ok {
		return nil, fmt.Errorf("mqtt connect timeout")
	}
	if err := connectToken.Error(); err != nil {
		return nil, fmt.Errorf("mqtt connect: %w", err)
	}
	logger.Printf("mqtt connected: broker=%s client_id=%s", cfg.EMQXBroker, cfg.EMQXClientID)

	go func() {
		<-ctx.Done()
		if client.IsConnected() {
			client.Disconnect(250)
		}
	}()

	cleanup := func() {
		if client.IsConnected() {
			client.Disconnect(250)
		}
	}
	return cleanup, nil
}
