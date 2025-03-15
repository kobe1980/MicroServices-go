// Package rabbit provides RabbitMQ client adapters for pub/sub messaging
package rabbit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kobe1980/microservices-go/internal/logger"
	amqp "github.com/rabbitmq/amqp091-go"
)

// SocketType defines the type of socket (PUB/SUB)
type SocketType string

const (
	// PUB is a publisher socket
	PUB SocketType = "PUB"
	// SUB is a subscriber socket
	SUB SocketType = "SUB"
)

// DataHandler is a function that processes received messages
type DataHandler func([]byte)

// Context represents a connection to RabbitMQ
type Context struct {
	URL        string
	connection *amqp.Connection
	channel    *amqp.Channel
	exchanges  map[string]bool
	mu         sync.Mutex
	ready      bool
	readyFunc  []func()
}

// Socket represents a connection to a RabbitMQ exchange for publishing or subscribing
type Socket struct {
	context   *Context
	socketType SocketType
	exchange  string
	queue     string
	consumers map[string]string // mapping of routing keys to consumer tags
	handlers  map[string]DataHandler
	mu        sync.Mutex
}

// NewContext creates a new RabbitMQ context
func NewContext(url string) *Context {
	ctx := &Context{
		URL:       url,
		exchanges: make(map[string]bool),
		readyFunc: make([]func(), 0),
	}

	// Connect asynchronously
	go func() {
		if err := ctx.connect(); err != nil {
			logger.Log("RabbitAdapter", "Connection", 
				fmt.Sprintf("Failed to connect: %s", err.Error()), logger.ERROR)
		}
	}()

	return ctx
}

// connect establishes a connection to RabbitMQ
func (c *Context) connect() error {
	var err error

	c.connection, err = amqp.Dial(c.URL)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	c.channel, err = c.connection.Channel()
	if err != nil {
		return fmt.Errorf("failed to open a channel: %w", err)
	}

	// Setup error handling
	go func() {
		connErrCh := c.connection.NotifyClose(make(chan *amqp.Error))
		for err := range connErrCh {
			logger.Log("RabbitAdapter", "Connection", 
				fmt.Sprintf("Connection error: %s", err.Error()), logger.ERROR)
			c.mu.Lock()
			c.ready = false
			c.mu.Unlock()

			// Try to reconnect
			for {
				time.Sleep(1 * time.Second)
				if err := c.connect(); err == nil {
					break
				}
			}
		}
	}()

	// Extract port from URL for logging
	url := c.URL
	port := "5672" // Default AMQP port
	
	// Try to extract port from URL if present
	for i := 0; i < len(url); i++ {
		if i+2 < len(url) && url[i:i+3] == "://" {
			colonIndex := -1
			for j := i + 3; j < len(url); j++ {
				if url[j] == ':' {
					colonIndex = j
				}
				if url[j] == '/' {
					break
				}
			}
			if colonIndex != -1 {
				slashIndex := len(url)
				for j := colonIndex; j < len(url); j++ {
					if url[j] == '/' {
						slashIndex = j
						break
					}
				}
				port = url[colonIndex+1:slashIndex]
			}
			break
		}
	}

	logger.Log("RabbitAdapter", "Connection", 
		fmt.Sprintf("Connected to RabbitMQ server on port %s", port), logger.INFO)

	c.mu.Lock()
	c.ready = true
	// Call all readyFunc callbacks
	for _, fn := range c.readyFunc {
		go fn()
	}
	c.mu.Unlock()

	return nil
}

// OnReady registers a callback function to be called when the connection is ready
func (c *Context) OnReady(fn func()) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.ready {
		go fn() // Call immediately if already ready
	} else {
		c.readyFunc = append(c.readyFunc, fn)
	}
}

// IsReady returns true if the connection is ready
func (c *Context) IsReady() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ready
}

// Close closes the connection
func (c *Context) Close() error {
	logger.Log("RabbitAdapter", "Connection", "Closing RabbitMQ connection", logger.INFO)

	// Mark as not ready to prevent new connections
	c.mu.Lock()
	c.ready = false
	c.mu.Unlock()
	
	// Close channel first
	if c.channel != nil {
		logger.Log("RabbitAdapter", "Connection", "Closing channel", logger.INFO)
		err := c.channel.Close()
		if err != nil {
			logger.Log("RabbitAdapter", "Connection", 
				fmt.Sprintf("Error closing channel: %s", err.Error()), logger.ERROR)
		}
		c.channel = nil
	}
	
	// Then close connection
	if c.connection != nil {
		logger.Log("RabbitAdapter", "Connection", "Closing connection", logger.INFO)
		err := c.connection.Close()
		if err != nil {
			logger.Log("RabbitAdapter", "Connection", 
				fmt.Sprintf("Error closing connection: %s", err.Error()), logger.ERROR)
			return err
		}
		c.connection = nil
	}
	
	logger.Log("RabbitAdapter", "Connection", "RabbitMQ connection closed", logger.INFO)
	return nil
}

// NewSocket creates a new socket of the specified type
func (c *Context) NewSocket(socketType SocketType) (*Socket, error) {
	if socketType != PUB && socketType != SUB {
		return nil, fmt.Errorf("invalid socket type: %s", socketType)
	}

	socket := &Socket{
		context:   c,
		socketType: socketType,
		consumers: make(map[string]string),
		handlers:  make(map[string]DataHandler),
	}

	return socket, nil
}

// Connect connects the socket to an exchange
func (s *Socket) Connect(exchange, routingKey string, callback func()) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If socket is not ready, wait for ready
	if !s.context.IsReady() {
		s.context.OnReady(func() {
			if err := s.Connect(exchange, routingKey, callback); err != nil {
				logger.Log("RabbitAdapter", "Socket", 
					fmt.Sprintf("Error connecting socket: %s", err.Error()), logger.ERROR)
			}
		})
		return nil
	}

	// Create the exchange if it doesn't exist yet
	err := s.context.channel.ExchangeDeclare(
		exchange, // name
		"topic",  // type
		false,    // durable
		false,    // auto-deleted
		false,    // internal
		false,    // no-wait
		nil,      // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	s.exchange = exchange

	// For subscribers, create a queue and bind to the routing key
	if s.socketType == SUB {
		q, err := s.context.channel.QueueDeclare(
			"",    // name - let server generate a name
			false, // durable
			true,  // delete when unused
			true,  // exclusive
			false, // no-wait
			nil,   // arguments
		)
		if err != nil {
			return fmt.Errorf("failed to declare queue: %w", err)
		}

		s.queue = q.Name

		err = s.context.channel.QueueBind(
			q.Name,     // queue name
			routingKey, // routing key
			exchange,   // exchange
			false,      // no-wait
			nil,        // arguments
		)
		if err != nil {
			return fmt.Errorf("failed to bind queue: %w", err)
		}

		// Register the routing key for message handling
		s.handlers[routingKey] = nil
	}

	// Call the callback to signal success
	if callback != nil {
		callback()
	}

	return nil
}

// On registers a handler for a specific event
func (s *Socket) On(event string, handler func([]byte)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if event != "data" {
		return fmt.Errorf("unsupported event: %s", event)
	}

	// Set handler for all registered routing keys
	for routingKey := range s.handlers {
		s.handlers[routingKey] = handler

		// Start consuming if not already
		if _, exists := s.consumers[routingKey]; !exists {
			go s.consume(routingKey, handler)
		}
	}

	return nil
}

// consume starts a goroutine to consume messages
func (s *Socket) consume(routingKey string, handler DataHandler) {
	// Generate a unique consumer tag
	consumerTag := fmt.Sprintf("%s-%d", routingKey, time.Now().UnixNano())
	
	msgs, err := s.context.channel.Consume(
		s.queue,      // queue
		consumerTag,  // consumer tag - unique identifier
		true,         // auto-ack
		false,        // exclusive
		false,        // no-local
		false,        // no-wait
		nil,          // args
	)
	if err != nil {
		logger.Log("RabbitAdapter", "Socket", 
			fmt.Sprintf("Failed to consume from queue: %s", err.Error()), logger.ERROR)
		return
	}
	
	// Store the consumer tag for later cancellation
	s.mu.Lock()
	s.consumers[routingKey] = consumerTag
	s.mu.Unlock()
	
	// Process messages until channel is closed
	for msg := range msgs {
		if handler != nil {
			handler(msg.Body)
		}
	}
	
	logger.Log("RabbitAdapter", "Socket", 
		fmt.Sprintf("Consumer %s for routing key %s has stopped", consumerTag, routingKey), logger.INFO)
}

// Publish publishes a message to the exchange with the specified routing key
func (s *Socket) Publish(routingKey string, data []byte) error {
	if s.exchange == "" {
		return fmt.Errorf("socket not connected to any exchange")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.context.channel.PublishWithContext(
		ctx,
		s.exchange, // exchange
		routingKey, // routing key
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			ContentType: "application/octet-stream",
			Body:        data,
		},
	)
}

// Emit is a higher-level method that serializes and publishes data
// This provides compatibility with the JavaScript version's socket.emit pattern
func (s *Socket) Emit(routingKey string, data interface{}, serializer func(interface{}) ([]byte, error)) error {
	if serializer == nil {
		return fmt.Errorf("serializer function is required")
	}

	// Serialize the data
	serialized, err := serializer(data)
	if err != nil {
		return fmt.Errorf("failed to serialize data: %w", err)
	}

	// Publish the serialized data
	return s.Publish(routingKey, serialized)
}

// EmitWithCompressor is a convenience helper that uses a compressor for serialization
// The compressor interface is a subset of the internal/compressor.Compressor
type CompressorInterface interface {
	Serialize(data interface{}) ([]byte, error)
}

// EmitWithCompressor emits a message using the provided compressor for serialization
func (s *Socket) EmitWithCompressor(routingKey string, data interface{}, compressor CompressorInterface) error {
	if compressor == nil {
		return fmt.Errorf("compressor is required")
	}
	
	return s.Emit(routingKey, data, compressor.Serialize)
}

// Close closes the socket and its subscriptions
func (s *Socket) Close() error {
	s.mu.Lock()

	// Log that we're closing the socket
	logger.Log("RabbitAdapter", "Socket", 
		fmt.Sprintf("Closing socket with %d consumers", len(s.consumers)), logger.INFO)

	// Cancel all consumers
	for routingKey, consumerTag := range s.consumers {
		logger.Log("RabbitAdapter", "Socket", 
			fmt.Sprintf("Canceling consumer %s for routing key %s", consumerTag, routingKey), logger.INFO)
		
		err := s.context.channel.Cancel(consumerTag, false)
		if err != nil {
			logger.Log("RabbitAdapter", "Socket", 
				fmt.Sprintf("Error canceling consumer %s: %s", routingKey, err.Error()), logger.ERROR)
		}
	}

	// Clear consumer map
	s.consumers = make(map[string]string)
	s.mu.Unlock()
	
	// Sleep briefly to allow consumers to process the cancellation
	time.Sleep(100 * time.Millisecond)
	
	return nil
}

// End is an alias for Close for compatibility with the original API
func (s *Socket) End() error {
	return s.Close()
}