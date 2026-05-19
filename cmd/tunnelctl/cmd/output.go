package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v3"
)

// ActionResponse is a standard structure for action commands to return structured data
type ActionResponse struct {
	Status  string `json:"status" yaml:"status"`
	Message string `json:"message" yaml:"message"`
}

// PrintOutput handles formatting and printing data based on the requested output format.
// If the format is 'text', it executes the textFallback function.
func PrintOutput(data interface{}, textFallback func()) {
	format := outputFormat
	if format == "" {
		format = "text"
	}

	switch format {
	case "json":
		if msg, ok := data.(proto.Message); ok {
			m := protojson.MarshalOptions{Multiline: true, Indent: "  "}
			b, err := m.Marshal(msg)
			if err != nil {
				FatalError("failed to marshal json", err)
			}
			fmt.Println(string(b))
			return
		}
		b, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			FatalError("failed to marshal json", err)
		}
		fmt.Println(string(b))
	case "yaml":
		// For proto messages, convert to JSON then to YAML to respect proto tags/names
		if msg, ok := data.(proto.Message); ok {
			m := protojson.MarshalOptions{}
			b, err := m.Marshal(msg)
			if err != nil {
				FatalError("failed to marshal json", err)
			}
			var obj interface{}
			if err := json.Unmarshal(b, &obj); err != nil {
				FatalError("failed to unmarshal json for yaml conversion", err)
			}
			data = obj
		}
		b, err := yaml.Marshal(data)
		if err != nil {
			FatalError("failed to marshal yaml", err)
		}
		fmt.Print(string(b))
	default:
		if textFallback != nil {
			textFallback()
		}
	}
}

// FatalError prints a formatted error and exits.
func FatalError(message string, err error) {
	fullMsg := message
	if err != nil {
		fullMsg = fmt.Sprintf("%s: %v", message, err)
	}

	format := outputFormat
	if format == "json" || format == "yaml" {
		PrintOutput(ActionResponse{Status: "error", Message: fullMsg}, nil)
		os.Exit(1)
	}

	log.Fatal(fullMsg)
}
