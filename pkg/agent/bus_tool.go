package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dh-kam/kakaotalk-dobby/pkg/academy"
)

// BusScheduleTool allows the Agent to look up multi-academy shuttle bus times.
type BusScheduleTool struct {
	svc *academy.Service
}

// NewBusScheduleTool creates a BusScheduleTool.
func NewBusScheduleTool(svc *academy.Service) *BusScheduleTool {
	return &BusScheduleTool{svc: svc}
}

func (t *BusScheduleTool) Name() string {
	return "get_bus_schedule"
}

func (t *BusScheduleTool) Description() string {
	return "Lookup academy shuttle bus departure and boarding schedules by academy name (e.g. 정상어학원, 강의하는아이들), boarding stop location (e.g. 우미린2차, 양포도서관, 해마루초), vehicle number (e.g. 2호차), or class time."
}

func (t *BusScheduleTool) ParametersSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"academy": map[string]interface{}{
				"type":        "string",
				"description": "Optional academy name keyword (e.g. '정상어학원', '강의하는아이들', '정상').",
			},
			"location": map[string]interface{}{
				"type":        "string",
				"description": "Optional boarding stop or apartment name (e.g. '우미린2차', '양포도서관', '해마루초', '현진', '이편한').",
			},
			"class_time": map[string]interface{}{
				"type":        "string",
				"description": "Optional class time filter (e.g. '3시 40분', '5시 20분', '15:40').",
			},
			"vehicle": map[string]interface{}{
				"type":        "string",
				"description": "Optional vehicle number (e.g. '1호차', '2호차').",
			},
		},
	}
}

func (t *BusScheduleTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	if t.svc == nil {
		return "", fmt.Errorf("bus schedule service is not configured")
	}

	var args struct {
		Academy   string `json:"academy"`
		Location  string `json:"location"`
		ClassTime string `json:"class_time"`
		Vehicle   string `json:"vehicle"`
	}

	if argsJSON != "" && argsJSON != "{}" {
		_ = json.Unmarshal([]byte(argsJSON), &args)
	}

	matches := t.svc.Search(academy.SearchQuery{
		Academy:   args.Academy,
		Location:  args.Location,
		ClassTime: args.ClassTime,
		Vehicle:   args.Vehicle,
	})

	if len(matches) == 0 {
		academies := t.svc.ListAcademies()
		return fmt.Sprintf("조회 조건에 맞는 버스 시간표 정보를 찾을 수 없습니다. (검색어: 학원=%q, 위치=%q, 차량=%q)\n등록된 학원 노선: %s",
			args.Academy, args.Location, args.Vehicle, strings.Join(academies, ", ")), nil
	}

	resBytes, err := json.MarshalIndent(matches, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal schedule results: %w", err)
	}

	return string(resBytes), nil
}
