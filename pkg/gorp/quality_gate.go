package gorp

import (
	"strconv"
)

type QualityGate struct {
	ID     int64
	Status string
}

func ParseQualityGate(metadata map[string]any) (*QualityGate, bool) {
	if metadata == nil {
		return nil, false
	}
	if qg, ok := metadata["qualityGate"].(map[string]any); ok {
		var qgID int64

		qgStatus, exists := qg["status"]
		if !exists {
			qgStatus = ""
		}

		switch qgIDType := qg["id"].(type) {
		case nil:
			return nil, false
		case int64:
			qgID = qgIDType
		case float64:
			qgID = int64(qgIDType)
		case string:
			if id, err := strconv.ParseInt(qgIDType, 10, 64); err == nil {
				qgID = id
			} else {
				return nil, false
			}
		}

		return &QualityGate{ID: qgID, Status: qgStatus.(string)}, true
	}
	return nil, false
}
