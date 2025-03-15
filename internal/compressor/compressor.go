package compressor

import (
	"encoding/json"
	"fmt"

	"github.com/kobe1980/microservices-go/internal/config"
	"github.com/vmihailenco/msgpack/v5"
	"go.mongodb.org/mongo-driver/bson"
)

// Compressor handles serializing and deserializing data
type Compressor struct {
	config *config.Config
}

// NewCompressor creates a new compressor with the given configuration
func NewCompressor(cfg *config.Config) *Compressor {
	return &Compressor{
		config: cfg,
	}
}

// Serialize serializes data to the configured format
func (c *Compressor) Serialize(data interface{}) ([]byte, error) {
	var err error
	var serialized []byte

	// Use the configured serialization format
	switch c.config.DataTransferProtocol {
	case "json":
		serialized, err = json.Marshal(data)
	case "bson":
		serialized, err = bson.Marshal(data)
	case "msgpack":
		serialized, err = msgpack.Marshal(data)
	default:
		// Default to JSON if format is unknown
		serialized, err = json.Marshal(data)
	}

	if err != nil {
		return nil, fmt.Errorf("serialize: %w", err)
	}

	return serialized, nil
}

// Deserialize deserializes data from the configured format
func (c *Compressor) Deserialize(data []byte, v interface{}) error {
	var err error

	// Use the configured deserialization format
	switch c.config.DataTransferProtocol {
	case "json":
		err = json.Unmarshal(data, v)
	case "bson":
		err = bson.Unmarshal(data, v)
	case "msgpack":
		err = msgpack.Unmarshal(data, v)
	default:
		// Default to JSON if format is unknown
		err = json.Unmarshal(data, v)
	}

	if err != nil {
		return fmt.Errorf("deserialize: %w", err)
	}

	return nil
}