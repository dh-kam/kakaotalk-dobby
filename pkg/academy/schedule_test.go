package academy

import (
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
