package academy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ClassInfo represents an academy class slot.
type ClassInfo struct {
	ClassID   string `json:"class_id"`
	ClassName string `json:"class_name"`
	ClassTime string `json:"class_time"`
}

// StopInfo represents a bus boarding location and arrival times.
type StopInfo struct {
	Sequence         int               `json:"sequence"`
	Location         string            `json:"location"`
	Aliases          []string          `json:"aliases,omitempty"`
	Highlighted      bool              `json:"highlighted,omitempty"`
	Note             string            `json:"note,omitempty"`
	Schedules        map[string]string `json:"schedules"`
	DisplaySchedules map[string]string `json:"display_schedules"`
}

// AcademyMetadata holds general academy and shuttle contact details.
type AcademyMetadata struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Aliases        []string `json:"aliases"`
	Contact        string   `json:"contact"`
	OperatingHours string   `json:"operating_hours"`
	VehicleNumber  string   `json:"vehicle_number"`
	Type           string   `json:"type"` // "등원", "하원"
}

// BusSchedule represents a complete shuttle route schedule for an academy.
type BusSchedule struct {
	Academy AcademyMetadata `json:"academy"`
	Classes []ClassInfo     `json:"classes"`
	Stops   []StopInfo      `json:"stops"`
	Notices []string        `json:"notices"`
}

// SearchQuery represents criteria for searching bus schedules.
type SearchQuery struct {
	Academy   string `json:"academy"`
	Location  string `json:"location"`
	ClassTime string `json:"class_time"`
	Vehicle   string `json:"vehicle"`
	Type      string `json:"type"`
}

// MatchResult represents a matched stop result.
type MatchResult struct {
	AcademyName    string            `json:"academy_name"`
	VehicleNumber  string            `json:"vehicle_number"`
	ScheduleType   string            `json:"schedule_type"`
	Location       string            `json:"location"`
	Contact        string            `json:"contact"`
	OperatingHours string            `json:"operating_hours"`
	Highlighted    bool              `json:"highlighted,omitempty"`
	Note           string            `json:"note,omitempty"`
	Times          map[string]string `json:"times"`
	Notices        []string          `json:"notices,omitempty"`
}

// Service manages multi-academy bus schedules.
type Service struct {
	schedules []*BusSchedule
	seen      map[string]bool
	mu        sync.RWMutex
}

// NewService creates a new BusSchedule Service.
func NewService() *Service {
	return &Service{
		schedules: make([]*BusSchedule, 0),
		seen:      make(map[string]bool),
	}
}

// LoadFromDir loads all schedule JSON files from a directory without duplicates.
func (s *Service) LoadFromDir(dirPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("read schedules dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		filePath := filepath.Join(dirPath, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var sched BusSchedule
		if err := json.Unmarshal(data, &sched); err != nil {
			continue
		}

		key := fmt.Sprintf("%s|%s|%s", sched.Academy.Name, sched.Academy.VehicleNumber, sched.Academy.Type)
		if s.seen[key] {
			continue
		}
		s.seen[key] = true
		s.schedules = append(s.schedules, &sched)
	}

	return nil
}

// ReloadFromDir clears in-memory schedules and reloads all schedule JSON files from the directory.
func (s *Service) ReloadFromDir(dirPath string) error {
	s.mu.Lock()
	s.schedules = make([]*BusSchedule, 0)
	s.seen = make(map[string]bool)
	s.mu.Unlock()

	return s.LoadFromDir(dirPath)
}

// AddSchedule adds an in-memory schedule without duplicates.
func (s *Service) AddSchedule(sched *BusSchedule) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := fmt.Sprintf("%s|%s|%s", sched.Academy.Name, sched.Academy.VehicleNumber, sched.Academy.Type)
	if s.seen[key] {
		return
	}
	s.seen[key] = true
	s.schedules = append(s.schedules, sched)
}

// Search searches schedules matching the query with deduplicated results.
func (s *Service) Search(q SearchQuery) []MatchResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []MatchResult
	seenStops := make(map[string]bool)

	academyQ := strings.ToLower(strings.TrimSpace(q.Academy))
	locationQ := strings.ToLower(strings.TrimSpace(q.Location))
	vehicleQ := strings.ToLower(strings.TrimSpace(q.Vehicle))
	typeQ := strings.ToLower(strings.TrimSpace(q.Type))

	for _, sched := range s.schedules {
		if academyQ != "" && !s.matchAcademy(sched.Academy, academyQ) {
			continue
		}
		if vehicleQ != "" && !strings.Contains(strings.ToLower(sched.Academy.VehicleNumber), vehicleQ) {
			continue
		}
		if typeQ != "" && !strings.Contains(strings.ToLower(sched.Academy.Type), typeQ) {
			continue
		}

		for _, stop := range sched.Stops {
			if locationQ != "" && !s.matchStopLocation(stop, locationQ) {
				continue
			}

			stopKey := fmt.Sprintf("%s|%s|%s", sched.Academy.Name, sched.Academy.VehicleNumber, stop.Location)
			if seenStops[stopKey] {
				continue
			}
			seenStops[stopKey] = true

			// Format display times
			times := make(map[string]string)
			for _, cls := range sched.Classes {
				val, ok := stop.DisplaySchedules[cls.ClassName]
				if !ok || val == "" {
					val = stop.Schedules[cls.ClassID]
				}
				if val != "" {
					times[cls.ClassName] = val
				}
			}

			results = append(results, MatchResult{
				AcademyName:    sched.Academy.Name,
				VehicleNumber:  sched.Academy.VehicleNumber,
				ScheduleType:   sched.Academy.Type,
				Location:       stop.Location,
				Contact:        sched.Academy.Contact,
				OperatingHours: sched.Academy.OperatingHours,
				Highlighted:    stop.Highlighted,
				Note:           stop.Note,
				Times:          times,
				Notices:        sched.Notices,
			})
		}
	}

	return results
}

// ListAcademies returns all registered academy names.
func (s *Service) ListAcademies() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var names []string
	for _, sched := range s.schedules {
		names = append(names, fmt.Sprintf("%s (%s %s)", sched.Academy.Name, sched.Academy.VehicleNumber, sched.Academy.Type))
	}
	return names
}

func (s *Service) matchAcademy(meta AcademyMetadata, q string) bool {
	if strings.Contains(strings.ToLower(meta.Name), q) {
		return true
	}
	for _, alias := range meta.Aliases {
		if strings.Contains(strings.ToLower(alias), q) {
			return true
		}
	}
	return false
}

func (s *Service) matchStopLocation(stop StopInfo, q string) bool {
	cleanLoc := strings.ReplaceAll(strings.ToLower(stop.Location), " ", "")
	cleanQ := strings.ReplaceAll(strings.ToLower(q), " ", "")

	if strings.Contains(cleanLoc, cleanQ) {
		return true
	}
	for _, alias := range stop.Aliases {
		cleanAlias := strings.ReplaceAll(strings.ToLower(alias), " ", "")
		if strings.Contains(cleanAlias, cleanQ) || strings.Contains(cleanQ, cleanAlias) {
			return true
		}
	}

	// Canonical apartment aliases
	if (cleanQ == "우미린더스카이" || cleanQ == "더스카이" || cleanQ == "센트럴파크") && strings.Contains(cleanLoc, "우미린2차") {
		return true
	}
	if (cleanQ == "우미린풀하우스" || cleanQ == "풀하우스") && strings.Contains(cleanLoc, "우미린1차") {
		return true
	}

	return false
}

// HasBusQuery checks if text matches common bus/shuttle keywords, or any registered academy/stop names or aliases.
func (s *Service) HasBusQuery(text string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clean := strings.ToLower(strings.TrimSpace(text))
	commonKeywords := []string{"버스", "시간표", "정류장", "등원", "하원", "기사", "셔틀", "차량", "탑승"}
	for _, kw := range commonKeywords {
		if strings.Contains(clean, kw) {
			return true
		}
	}

	for _, sched := range s.schedules {
		if s.matchAcademy(sched.Academy, clean) {
			return true
		}
		for _, stop := range sched.Stops {
			if s.matchStopLocation(stop, clean) {
				return true
			}
		}
	}
	return false
}
