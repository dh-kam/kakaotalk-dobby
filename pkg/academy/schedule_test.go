package academy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAcademyService_Search(t *testing.T) {
	svc := NewService()
	svc.AddSchedule(&BusSchedule{
		Academy: AcademyMetadata{
			ID:            "jungsang",
			Name:          "산동옥계 정상어학원",
			Aliases:       []string{"정상어학원", "정상"},
			Contact:       "010-3807-7000",
			VehicleNumber: "2호차",
			Type:          "등원",
		},
		Classes: []ClassInfo{
			{ClassID: "c1", ClassName: "3시 40분 수업", ClassTime: "15:40"},
			{ClassID: "c2", ClassName: "5시 20분 수업", ClassTime: "17:20"},
		},
		Stops: []StopInfo{
			{
				Sequence: 1,
				Location: "우미린2차 정문 승강장",
				DisplaySchedules: map[string]string{
					"3시 40분 수업": "3:10",
					"5시 20분 수업": "4:50",
				},
			},
			{
				Sequence: 2,
				Location: "양포도서관 앞 대로변",
				DisplaySchedules: map[string]string{
					"3시 40분 수업": "3:21",
					"5시 20분 수업": "5:01",
				},
			},
		},
		Notices: []string{"3분 전 대기"},
	})

	// Search by academy & location
	res := svc.Search(SearchQuery{
		Academy:  "정상",
		Location: "우미린",
	})
	require.Len(t, res, 1)
	assert.Equal(t, "우미린2차 정문 승강장", res[0].Location)
	assert.Equal(t, "3:10", res[0].Times["3시 40분 수업"])
	assert.Equal(t, "010-3807-7000", res[0].Contact)

	// Search by non-existent location
	resNone := svc.Search(SearchQuery{
		Academy:  "정상",
		Location: "강남역",
	})
	assert.Empty(t, resNone)
}

func TestAcademyService_ReloadAndHasBusQuery(t *testing.T) {
	svc := NewService()
	dir := "../../data/schedules"
	err := svc.LoadFromDir(dir)
	require.NoError(t, err)

	assert.True(t, svc.HasBusQuery("정상어학원 버스 언제 와?"))
	assert.True(t, svc.HasBusQuery("우미린2차 버스 몇 시?"))
	assert.True(t, svc.HasBusQuery("셔틀 시간표 알려줘"))
	assert.False(t, svc.HasBusQuery("오늘 날씨 어때?"))

	// Test reload
	err = svc.ReloadFromDir(dir)
	require.NoError(t, err)
	assert.True(t, len(svc.ListAcademies()) > 0)
}

func TestAcademyService_LoadClassesOnlyFile(t *testing.T) {
	dir := t.TempDir()
	classesOnly := `{"academy":{"id":"a1","name":"코딩학원","vehicle_number":"12가3456","type":"등원"},"classes":[{"class_id":"c1","class_name":"A반","class_time":"15:00"}]}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "classes_only.json"), []byte(classesOnly), 0o644))

	svc := NewService()
	require.NoError(t, svc.LoadFromDir(dir))
	assert.Equal(t, []string{"코딩학원 (12가3456 등원)"}, svc.ListAcademies())
}

func TestAcademyService_SearchPrefersDisplaySchedules(t *testing.T) {
	svc := NewService()
	svc.AddSchedule(&BusSchedule{
		Academy: AcademyMetadata{Name: "테스트학원", VehicleNumber: "12가3456", Type: "등원"},
		Classes: []ClassInfo{{ClassID: "class_a", ClassName: "A반"}},
		Stops: []StopInfo{{
			Location:         "테스트정류장",
			Schedules:        map[string]string{"class_a": "15:00"},
			DisplaySchedules: map[string]string{"A반": "오후 3시"},
		}},
	})

	results := svc.Search(SearchQuery{})
	require.Len(t, results, 1)
	assert.Equal(t, map[string]string{"A반": "오후 3시"}, results[0].Times)
}

func TestAcademyService_SearchFallsBackToRawSchedulesWithoutDisplay(t *testing.T) {
	svc := NewService()
	svc.AddSchedule(&BusSchedule{
		Academy: AcademyMetadata{Name: "테스트학원", VehicleNumber: "12가3456", Type: "등원"},
		Classes: []ClassInfo{{ClassID: "class_a", ClassName: "A반"}},
		Stops: []StopInfo{{
			Location:  "테스트정류장",
			Schedules: map[string]string{"class_a": "15:00"},
		}},
	})

	results := svc.Search(SearchQuery{})
	require.Len(t, results, 1)
	assert.Equal(t, map[string]string{"A반": "15:00"}, results[0].Times)
}
