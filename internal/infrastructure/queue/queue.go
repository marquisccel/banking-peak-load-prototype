package queue

import (
	"context"
	"fmt"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Client struct {
	url     string
	mu      sync.Mutex
	conn    *amqp.Connection
	channel *amqp.Channel
}

func New(url string) (*Client, error) {
	c := &Client{url: url}
	if err := c.dial(); err != nil {
		return nil, err
	}
	return c, nil
}

// dial opens a fresh connection and channel, then declares the required queues.
// Callers must hold c.mu.
func (c *Client) dial() error {
	conn, err := amqp.Dial(c.url)
	if err != nil {
		return fmt.Errorf("amqp dial: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("open channel: %w", err)
	}
	if err = declareQueues(ch); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return err
	}
	c.conn = conn
	c.channel = ch
	return nil
}

func declareQueues(ch *amqp.Channel) error {
	if _, err := ch.QueueDeclare("transactions.dlq", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare dlq: %w", err)
	}
	args := amqp.Table{
		"x-dead-letter-exchange":    "",
		"x-dead-letter-routing-key": "transactions.dlq",
	}
	if _, err := ch.QueueDeclare("transactions", true, false, false, false, args); err != nil {
		return fmt.Errorf("declare transactions queue: %w", err)
	}
	return nil
}

// getChannel returns the active channel, re-dialing if the connection or channel is closed.
func (c *Client) getChannel() (*amqp.Channel, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil && !c.conn.IsClosed() && c.channel != nil {
		return c.channel, nil
	}
	if err := c.dial(); err != nil {
		return nil, fmt.Errorf("reconnect: %w", err)
	}
	return c.channel, nil
}

func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.channel != nil {
		_ = c.channel.Close()
	}
	if c.conn != nil {
		_ = c.conn.Close()
	}
}

func (c *Client) Publish(ctx context.Context, queue string, body []byte) error {
	ch, err := c.getChannel()
	if err != nil {
		return err
	}
	return ch.PublishWithContext(ctx, "", queue, false, false,
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
		})
}

// Consume registers a consumer on the given queue and returns the delivery channel.
func (c *Client) Consume(queue string) (<-chan amqp.Delivery, error) {
	ch, err := c.getChannel()
	if err != nil {
		return nil, err
	}
	return ch.Consume(queue, "", false, false, false, false, nil)
}

// QueueDepth returns the number of messages currently waiting in the given queue.
func (c *Client) QueueDepth(queue string) (int64, error) {
	ch, err := c.getChannel()
	if err != nil {
		return 0, err
	}
	q, err := ch.QueueInspect(queue)
	if err != nil {
		return 0, fmt.Errorf("inspect queue %s: %w", queue, err)
	}
	return int64(q.Messages), nil
}
