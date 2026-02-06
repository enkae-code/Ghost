// Author: Enkae (enkae.dev@pm.me)
package adapter

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"ghost/kernel/internal/domain"
)

// SentinelProcess manages the Rust Sentinel subprocess
type SentinelProcess struct {
	cmd  *exec.Cmd
	path string
}

// NewSentinelProcess creates a new SentinelProcess instance
func NewSentinelProcess() *SentinelProcess {
	workDir, _ := os.Getwd()
	sentinelPath := filepath.Join(filepath.Dir(workDir), "target", "debug", "engram-sentinel.exe")

	return &SentinelProcess{
		path: sentinelPath,
	}
}

// Start launches the sentinel process and returns a channel for receiving artifacts
func (sp *SentinelProcess) Start() (<-chan domain.Artifact, error) {
	// Create the command
	sp.cmd = exec.Command(sp.path)

	// Get stdout pipe
	stdout, err := sp.cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	fmt.Printf("[DEBUG] Launching Sentinel Binary at: %s\n", sp.cmd.Path)

	// Start the process
	if err := sp.cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start sentinel process: %w", err)
	}

	log.Printf("Started sentinel process (PID: %d)", sp.cmd.Process.Pid)

	// Create channel for artifacts
	artifactChan := make(chan domain.Artifact, 100)

	// Start goroutine to read stdout and parse JSON
	go sp.readOutput(stdout, artifactChan)

	// Start goroutine to wait for process completion
	go sp.waitForCompletion()

	return artifactChan, nil
}

// readOutput reads the sentinel's stdout line by line and parses JSON artifacts
func (sp *SentinelProcess) readOutput(stdout interface{}, artifactChan chan<- domain.Artifact) {
	defer close(artifactChan)

	scanner := bufio.NewScanner(stdout.(interface {
		Read([]byte) (int, error)
	}))

	for scanner.Scan() {
		line := scanner.Text()

		// Parse JSON from Rust sentinel
		var uiElement struct {
			Name              string        `json:"name"`
			ControlType       string        `json:"control_type"`
			BoundingRectangle string        `json:"bounding_rectangle"`
			Children          []interface{} `json:"children"`
		}

		if err := json.Unmarshal([]byte(line), &uiElement); err != nil {
			log.Printf("Failed to parse JSON from sentinel: %v (line: %s)", err, line)
			continue
		}

		// Convert to domain.Artifact
		artifact := sp.convertToArtifact(uiElement)

		// Send artifact to channel
		select {
		case artifactChan <- artifact:
		default:
			log.Printf("Artifact channel full, dropping artifact: %s", artifact.Content)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading sentinel output: %v", err)
	}
}

// convertToArtifact converts a UIElement to a domain.Artifact
func (sp *SentinelProcess) convertToArtifact(uiElement struct {
	Name              string        `json:"name"`
	ControlType       string        `json:"control_type"`
	BoundingRectangle string        `json:"bounding_rectangle"`
	Children          []interface{} `json:"children"`
}) domain.Artifact {
	// Map control type to artifact type
	artifactType := sp.mapControlTypeToArtifactType(uiElement.ControlType)

	// Parse bounding rectangle string
	boundingBox := sp.parseBoundingRectangle(uiElement.BoundingRectangle)

	return domain.NewArtifact(artifactType, uiElement.Name, boundingBox)
}

// parseBoundingRectangle parses the bounding rectangle string from Rust
// Format: "left=X,top=Y,right=Z,bottom=W"
func (sp *SentinelProcess) parseBoundingRectangle(rectStr string) domain.BoundingBox {
	var left, top, right, bottom int
	
	// Parse the string format: "left=X,top=Y,right=Z,bottom=W"
	_, err := fmt.Sscanf(rectStr, "left=%d,top=%d,right=%d,bottom=%d", &left, &top, &right, &bottom)
	if err != nil {
		log.Printf("Failed to parse bounding rectangle: %v", err)
		return domain.BoundingBox{Left: 0, Top: 0, Right: 0, Bottom: 0}
	}
	
	return domain.BoundingBox{
		Left:   left,
		Top:    top,
		Right:  right,
		Bottom: bottom,
	}
}

// mapControlTypeToArtifactType maps UI control types to domain artifact types
func (sp *SentinelProcess) mapControlTypeToArtifactType(controlType string) domain.ArtifactType {
	switch controlType {
	case "window":
		return domain.ArtifactTypeWindow
	case "button":
		return domain.ArtifactTypeButton
	case "text", "document":
		return domain.ArtifactTypeText
	case "edit":
		return domain.ArtifactTypeEdit
	case "list":
		return domain.ArtifactTypeList
	case "menu item":
		return domain.ArtifactTypeMenuItem
	default:
		return domain.ArtifactTypeUnknown
	}
}

// waitForCompletion waits for the sentinel process to finish
func (sp *SentinelProcess) waitForCompletion() {
	err := sp.cmd.Wait()
	if err != nil {
		log.Printf("Sentinel process finished with error: %v", err)
	} else {
		log.Println("Sentinel process finished successfully")
	}
}

// Stop terminates the sentinel process
func (sp *SentinelProcess) Stop() error {
	if sp.cmd != nil && sp.cmd.Process != nil {
		log.Println("Stopping sentinel process...")
		return sp.cmd.Process.Kill()
	}
	return nil
}

// Wait blocks until the sentinel process finishes
func (sp *SentinelProcess) Wait() error {
	if sp.cmd != nil {
		return sp.cmd.Wait()
	}
	return nil
}
