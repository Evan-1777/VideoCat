package pipeline

import (
	"encoding/json"
	"os"

	"videocat/internal/config"
	"videocat/internal/probe"
)

// MetaRecord 是写入 <源名>.cat.meta.json 的内容。
type MetaRecord struct {
	Source           probe.VideoMeta    `json:"source"`
	Params           config.Params      `json:"params"`
	Encoder          string             `json:"encoder"`
	HardwareDecode   bool               `json:"hardwareDecode"`
	Outputs          Outputs            `json:"outputs"`
	SourceDuration   float64            `json:"sourceDuration"`
	OutputDuration   float64            `json:"outputDuration"`
	CompressionRatio float64            `json:"compressionRatio"`
	PhaseSeconds     map[string]float64 `json:"phaseSeconds"`
	AudioSkipped     bool               `json:"audioSkipped"`
}

// WriteMeta 将任务元数据写入 path（同名直接覆盖）。
func WriteMeta(path string, rec MetaRecord) error {
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
