package rabbit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewContext(t *testing.T) {
	// Create a new context with a non-existent URL
	// We can't test an actual connection without RabbitMQ
	ctx := NewContext("amqp://nonexistent")
	
	// Verify context creation
	assert.NotNil(t, ctx)
	assert.Equal(t, "amqp://nonexistent", ctx.URL)
	assert.NotNil(t, ctx.exchanges)
	assert.NotNil(t, ctx.readyFunc)
	assert.False(t, ctx.ready)
	
	// Clean up
	ctx.Close()
}

func TestContextOnReady(t *testing.T) {
	// Create a new context
	ctx := NewContext("amqp://nonexistent")
	defer ctx.Close()
	
	// Setup callback flags
	called := false
	
	// Register callback
	ctx.OnReady(func() {
		called = true
	})
	
	// Manually set ready state to trigger callback
	ctx.mu.Lock()
	ctx.ready = true
	// Call all readyFunc callbacks
	for _, fn := range ctx.readyFunc {
		// Call directly instead of in goroutine for testing
		fn()
	}
	ctx.mu.Unlock()
	
	// Verify callback was called
	assert.True(t, called)
	
	// Test registering when already ready
	called2 := false
	ctx.OnReady(func() {
		called2 = true
	})
	
	// Verify callback was called immediately
	time.Sleep(10 * time.Millisecond) // Allow goroutine to execute
	assert.True(t, called2)
}

func TestContextIsReady(t *testing.T) {
	// Create a new context
	ctx := NewContext("amqp://nonexistent")
	defer ctx.Close()
	
	// Verify initial state
	assert.False(t, ctx.IsReady())
	
	// Set ready state
	ctx.mu.Lock()
	ctx.ready = true
	ctx.mu.Unlock()
	
	// Verify updated state
	assert.True(t, ctx.IsReady())
}

func TestNewSocket(t *testing.T) {
	// Create a new context
	ctx := NewContext("amqp://nonexistent")
	defer ctx.Close()
	
	// Create a PUB socket
	pubSocket, err := ctx.NewSocket(PUB)
	assert.NoError(t, err)
	assert.NotNil(t, pubSocket)
	assert.Equal(t, PUB, pubSocket.socketType)
	assert.Equal(t, ctx, pubSocket.context)
	assert.NotNil(t, pubSocket.consumers)
	assert.NotNil(t, pubSocket.handlers)
	
	// Create a SUB socket
	subSocket, err := ctx.NewSocket(SUB)
	assert.NoError(t, err)
	assert.NotNil(t, subSocket)
	assert.Equal(t, SUB, subSocket.socketType)
	
	// Try to create an invalid socket
	invalidSocket, err := ctx.NewSocket("INVALID")
	assert.Error(t, err)
	assert.Nil(t, invalidSocket)
}

func TestSocketConnect(t *testing.T) {
	// Create a new context
	ctx := NewContext("amqp://nonexistent")
	defer ctx.Close()
	
	// Create a socket
	socket, _ := ctx.NewSocket(PUB)
	
	// Setup callback flags
	called := false
	callbackFunc := func() {
		called = true
	}
	
	// Try to connect with context not ready
	err := socket.Connect("test-exchange", "test-key", callbackFunc)
	assert.NoError(t, err) // Should not error, but will queue callback
	
	// Verify callback wasn't called yet
	assert.False(t, called)
	
	// Trying to publish before connected should error
	err = socket.Publish("test-key", []byte("test"))
	assert.Error(t, err)
}

func TestSocketOn(t *testing.T) {
	// Create a new context
	ctx := NewContext("amqp://nonexistent")
	defer ctx.Close()
	
	// Create a socket
	socket, _ := ctx.NewSocket(SUB)
	
	// Try to register unsupported event
	err := socket.On("unsupported", func([]byte) {})
	assert.Error(t, err)
	
	// Make the socket look connected by setting handler keys
	socket.handlers = map[string]DataHandler{
		"test-key": nil,
	}
	
	// Register data handler
	err = socket.On("data", func(data []byte) {
		// Handler for testing
	})
	assert.NoError(t, err)
	
	// Verify handler was set
	assert.NotNil(t, socket.handlers["test-key"])
}

func TestSocketClose(t *testing.T) {
	// Create a new context
	ctx := NewContext("amqp://nonexistent")
	defer ctx.Close()
	
	// Create a socket
	socket, _ := ctx.NewSocket(SUB)
	
	// Set empty consumers map (mock it instead of using real consumers)
	socket.consumers = map[string]string{}
	
	// Close the socket
	err := socket.Close()
	assert.NoError(t, err)
	
	// Verify consumers were cleared
	assert.Empty(t, socket.consumers)
}

func TestSocketEnd(t *testing.T) {
	// Create a new context
	ctx := NewContext("amqp://nonexistent")
	defer ctx.Close()
	
	// Create a socket
	socket, _ := ctx.NewSocket(SUB)
	
	// Set empty consumers map (mock it instead of using real consumers)
	socket.consumers = map[string]string{}
	
	// End the socket
	err := socket.End()
	assert.NoError(t, err)
	
	// Verify consumers were cleared
	assert.Empty(t, socket.consumers)
}