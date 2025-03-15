package compressor

import (
	"testing"

	"github.com/kobe1980/microservices-go/internal/config"
)

// TestData is a struct to test serialization/deserialization
type TestData struct {
	Name  string `json:"name" bson:"name" msgpack:"name"`
	Value int    `json:"value" bson:"value" msgpack:"value"`
	Flag  bool   `json:"flag" bson:"flag" msgpack:"flag"`
}

func TestNewCompressor(t *testing.T) {
	cfg := config.DefaultConfig()
	c := NewCompressor(cfg)
	
	if c == nil {
		t.Error("NewCompressor returned nil")
	}
	
	if c.config != cfg {
		t.Error("Compressor did not store the config correctly")
	}
}

func TestSerializeDeserializeJSON(t *testing.T) {
	// Create a compressor with JSON configuration
	cfg := config.DefaultConfig()
	cfg.DataTransferProtocol = "json"
	c := NewCompressor(cfg)
	
	// Test data
	testData := TestData{
		Name:  "test",
		Value: 42,
		Flag:  true,
	}
	
	// Serialize
	serialized, err := c.Serialize(testData)
	if err != nil {
		t.Fatalf("Failed to serialize: %v", err)
	}
	
	// Deserialize
	var deserialized TestData
	err = c.Deserialize(serialized, &deserialized)
	if err != nil {
		t.Fatalf("Failed to deserialize: %v", err)
	}
	
	// Verify
	if deserialized.Name != testData.Name {
		t.Errorf("Expected Name to be '%s', got '%s'", testData.Name, deserialized.Name)
	}
	
	if deserialized.Value != testData.Value {
		t.Errorf("Expected Value to be %d, got %d", testData.Value, deserialized.Value)
	}
	
	if deserialized.Flag != testData.Flag {
		t.Errorf("Expected Flag to be %v, got %v", testData.Flag, deserialized.Flag)
	}
}

func TestSerializeDeserializeMsgpack(t *testing.T) {
	// Create a compressor with MessagePack configuration
	cfg := config.DefaultConfig()
	cfg.DataTransferProtocol = "msgpack"
	c := NewCompressor(cfg)
	
	// Test data
	testData := TestData{
		Name:  "test",
		Value: 42,
		Flag:  true,
	}
	
	// Serialize
	serialized, err := c.Serialize(testData)
	if err != nil {
		t.Fatalf("Failed to serialize: %v", err)
	}
	
	// Deserialize
	var deserialized TestData
	err = c.Deserialize(serialized, &deserialized)
	if err != nil {
		t.Fatalf("Failed to deserialize: %v", err)
	}
	
	// Verify
	if deserialized.Name != testData.Name {
		t.Errorf("Expected Name to be '%s', got '%s'", testData.Name, deserialized.Name)
	}
	
	if deserialized.Value != testData.Value {
		t.Errorf("Expected Value to be %d, got %d", testData.Value, deserialized.Value)
	}
	
	if deserialized.Flag != testData.Flag {
		t.Errorf("Expected Flag to be %v, got %v", testData.Flag, deserialized.Flag)
	}
}

func TestSerializeDeserializeBSON(t *testing.T) {
	// Create a compressor with BSON configuration
	cfg := config.DefaultConfig()
	cfg.DataTransferProtocol = "bson"
	c := NewCompressor(cfg)
	
	// Test data
	testData := TestData{
		Name:  "test",
		Value: 42,
		Flag:  true,
	}
	
	// Serialize
	serialized, err := c.Serialize(testData)
	if err != nil {
		t.Fatalf("Failed to serialize: %v", err)
	}
	
	// Deserialize
	var deserialized TestData
	err = c.Deserialize(serialized, &deserialized)
	if err != nil {
		t.Fatalf("Failed to deserialize: %v", err)
	}
	
	// Verify
	if deserialized.Name != testData.Name {
		t.Errorf("Expected Name to be '%s', got '%s'", testData.Name, deserialized.Name)
	}
	
	if deserialized.Value != testData.Value {
		t.Errorf("Expected Value to be %d, got %d", testData.Value, deserialized.Value)
	}
	
	if deserialized.Flag != testData.Flag {
		t.Errorf("Expected Flag to be %v, got %v", testData.Flag, deserialized.Flag)
	}
}

func TestSerializeDeserializeUnknownFormat(t *testing.T) {
	// Create a compressor with unknown format (should default to JSON)
	cfg := config.DefaultConfig()
	cfg.DataTransferProtocol = "unknown"
	c := NewCompressor(cfg)
	
	// Test data
	testData := TestData{
		Name:  "test",
		Value: 42,
		Flag:  true,
	}
	
	// Serialize
	serialized, err := c.Serialize(testData)
	if err != nil {
		t.Fatalf("Failed to serialize: %v", err)
	}
	
	// Deserialize
	var deserialized TestData
	err = c.Deserialize(serialized, &deserialized)
	if err != nil {
		t.Fatalf("Failed to deserialize: %v", err)
	}
	
	// Verify
	if deserialized.Name != testData.Name {
		t.Errorf("Expected Name to be '%s', got '%s'", testData.Name, deserialized.Name)
	}
	
	if deserialized.Value != testData.Value {
		t.Errorf("Expected Value to be %d, got %d", testData.Value, deserialized.Value)
	}
	
	if deserialized.Flag != testData.Flag {
		t.Errorf("Expected Flag to be %v, got %v", testData.Flag, deserialized.Flag)
	}
}