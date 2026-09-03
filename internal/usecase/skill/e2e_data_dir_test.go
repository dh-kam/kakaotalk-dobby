package skill

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dh-kam/kakaotalk-dobby/pkg/academy"
	"github.com/dh-kam/kakaotalk-dobby/pkg/agent"
	"github.com/dh-kam/kakaotalk-dobby/pkg/ai"
	"github.com/dh-kam/kakaotalk-dobby/pkg/openbuilder"
	"github.com/dh-kam/kakaotalk-dobby/pkg/school"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2E_DataDirDynamicIngestionAndQuery(t *testing.T) {
	// 1. Setup isolated data directory
	tempDataDir, err := os.MkdirTemp("", "kakaobot-e2e-datadir-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(tempDataDir) })

	adminToken := "e2e-admin-secret-token"
	serverAddr := "127.0.0.1:18099"

	busSvc := academy.NewService()
	schoolSvc := school.NewService()

	// Initial load from empty dir
	require.NoError(t, busSvc.LoadFromDir(tempDataDir))
	require.NoError(t, schoolSvc.LoadFromDir(tempDataDir))

	// Setup Agent with dynamic PromptFunc
	registry := agent.NewToolRegistry()
	registry.Register(agent.NewBusScheduleTool(busSvc))
	registry.Register(agent.NewSchoolTimetableTool(schoolSvc))

	mockAI := ai.NewMockProvider("mock-ai")
	botAgent := agent.NewAgent(agent.AgentConfig{
		Provider: agent.NewMockLLMProvider("mock-agent"),
		Tools:    registry,
		PromptFunc: func() string {
			return agent.BuildDomainKnowledge(busSvc, schoolSvc)
		},
		MaxIterations: 3,
	})

	var serverOutput bytes.Buffer
	useCase := NewSkillServeUseCase()

	ctx, cancel := contextWithTimeout(t, 15*time.Second)
	defer cancel()

	go func() {
		_ = useCase.Execute(ctx, SkillServeRequest{
			ListenAddr:    serverAddr,
			ChannelID:     "0xc0de1ab",
			AIProvider:    mockAI,
			Agent:         botAgent,
			BusService:    busSvc,
			SchoolService: schoolSvc,
			DataDir:       tempDataDir,
			AdminToken:    adminToken,
			SessionStore:  agent.NewMemorySessionStore(50),
			Out:           &serverOutput,
		})
	}()

	baseURL := "http://" + serverAddr
	waitForServer(t, baseURL+"/healthz")

	// =========================================================================
	// Phase 1: Baseline Verification before any uploads
	// =========================================================================
	// Initially, catalogs should be empty
	catResp := getCatalogs(t, baseURL, adminToken)
	assert.Equal(t, "success", catResp.Status)
	assert.Empty(t, catResp.Files)

	// User query for 태권도 when no data exists should not return bus schedule
	initialBusResp := sendSkillMessage(t, baseURL, "태권도 우미린 2차 버스 몇 시야?")
	assert.NotEmpty(t, initialBusResp.Template.Outputs)
	// Shouldn't have matched bus schedule fast path since 태권도 isn't registered
	assert.NotContains(t, getResponseText(initialBusResp), "용인대 태권도")

	// =========================================================================
	// Phase 2: Dynamic Upload of new datasets via POST /api/data/upload
	// =========================================================================
	// 2.1 Upload new Academy Bus Schedule (Multipart Form-Data)
	taekwondoJSON := `{
		"academy": {
			"id": "taekwondo-yongin",
			"name": "용인대 명품 태권도",
			"aliases": ["용인대 태권도", "태권도", "태권도학원"],
			"contact": "010-9999-8888",
			"vehicle_number": "1호차",
			"type": "등원"
		},
		"classes": [
			{"class_id": "c1", "class_name": "초등 저학년부", "class_time": "14:30"},
			{"class_id": "c2", "class_name": "초등 고학년부", "class_time": "16:00"}
		],
		"stops": [
			{
				"sequence": 1,
				"location": "우미린 2차 정문 승강장",
				"aliases": ["우미린 2차", "우미린2차"],
				"display_schedules": {
					"초등 저학년부": "14:15",
					"초등 고학년부": "15:45"
				},
				"note": "신호등 앞 정차"
			},
			{
				"sequence": 2,
				"location": "양포도서관 앞",
				"aliases": ["양포도서관", "양포"],
				"display_schedules": {
					"초등 저학년부": "14:20",
					"초등 고학년부": "15:50"
				}
			}
		],
		"notices": ["차량 탑승 3분 전 대기 바랍니다."]
	}`

	uploadResp1 := uploadFileMultipart(t, baseURL, adminToken, "태권도_1호차_등원.json", []byte(taekwondoJSON))
	assert.Equal(t, http.StatusOK, uploadResp1.StatusCode)
	var res1 adminResponse
	require.NoError(t, json.NewDecoder(uploadResp1.Body).Decode(&res1))
	uploadResp1.Body.Close()
	assert.Equal(t, "success", res1.Status)
	assert.Equal(t, "academy_bus_schedule", res1.DataType)
	assert.Equal(t, "태권도_1호차_등원.json", res1.Filename)

	// 2.2 Upload new School Timetable (Raw JSON Payload with ?filename=...)
	schoolJSON := `{
		"title": "2026학년도 구미원당초등학교 4학년 2반 시간표",
		"school_year": 2026,
		"grade": 4,
		"class_number": 2,
		"description": "2026학년도 4학년 2반 정규 주간 시간표",
		"weekly_timetable": {
			"monday": {
				"day_name": "월요일",
				"day_short": "월",
				"total_periods": 5,
				"dismissal_time": "14:10",
				"schedule": [
					{"period": 1, "time": "09:00~09:40", "subject": "도덕"},
					{"period": 2, "time": "09:50~10:30", "subject": "사회"},
					{"period": 3, "time": "10:40~11:20", "subject": "수학"},
					{"period": 4, "time": "11:30~12:10", "subject": "체육"},
					{"period": 5, "time": "13:00~13:40", "subject": "음악"}
				]
			}
		},
		"class_rules": {
			"title": "4학년 2반 교실 생활 수칙",
			"rules": [
				"친구에게 바르고 고운 말 쓰기",
				"복도에서 뛰지 않고 안전하게 걷기"
			]
		}
	}`

	uploadResp2 := uploadFileRawJSON(t, baseURL, adminToken, "2026_4-2_school_timetable.json", []byte(schoolJSON))
	assert.Equal(t, http.StatusOK, uploadResp2.StatusCode)
	var res2 adminResponse
	require.NoError(t, json.NewDecoder(uploadResp2.Body).Decode(&res2))
	uploadResp2.Body.Close()
	assert.Equal(t, "success", res2.Status)
	assert.Equal(t, "school_timetable", res2.DataType)

	// =========================================================================
	// Phase 3: Verify Catalog Reflects Ingested Data
	// =========================================================================
	catResp2 := getCatalogs(t, baseURL, adminToken)
	assert.Equal(t, "success", catResp2.Status)
	assert.ElementsMatch(t, []string{"태권도_1호차_등원.json", "2026_4-2_school_timetable.json"}, catResp2.Files)
	assert.Contains(t, catResp2.Catalogs["schoolTimetables"], "4학년 2반")

	// =========================================================================
	// Phase 4: Query newly uploaded data through Kakao OpenBuilder (/skill)
	// =========================================================================
	// 4.1 Query Bus Schedule for the newly uploaded Academy
	busQueryResp := sendSkillMessage(t, baseURL, "태권도 우미린 2차 버스 몇 시에 와?")
	busText := getResponseText(busQueryResp)
	t.Logf("Bus Query Response: %s", busText)

	assert.Contains(t, busText, "용인대 명품 태권도")
	assert.Contains(t, busText, "우미린 2차")
	assert.Contains(t, busText, "14:15")
	assert.Contains(t, busText, "010-9999-8888")

	// 4.2 Query School Timetable for the newly uploaded Class
	schoolQueryResp := sendSkillMessage(t, baseURL, "월요일 학교 시간표 알려줘")
	schoolText := getResponseText(schoolQueryResp)
	t.Logf("School Query Response: %s", schoolText)

	assert.Contains(t, schoolText, "월요일")
	assert.Contains(t, schoolText, "도덕")
	assert.Contains(t, schoolText, "사회")
	assert.Contains(t, schoolText, "수학")
	assert.Contains(t, schoolText, "체육")
	assert.Contains(t, schoolText, "음악")
	assert.Contains(t, schoolText, "14:10")

	// 4.3 Query Classroom Rules
	rulesResp := sendSkillMessage(t, baseURL, "학급 생활 규칙 알려줘")
	rulesText := getResponseText(rulesResp)
	t.Logf("Rules Query Response: %s", rulesText)

	assert.Contains(t, rulesText, "4학년 2반")
	assert.Contains(t, rulesText, "친구에게 바르고 고운 말 쓰기")

	// 4.4 Agent Tools & Dynamic Prompt Verification
	// Test that Autonomous Agent tools and prompts dynamically inspect the uploaded data
	busTool := agent.NewBusScheduleTool(busSvc)
	busToolOutput, err := busTool.Execute(t.Context(), `{"academy": "태권도", "location": "우미린2차"}`)
	require.NoError(t, err)
	assert.Contains(t, busToolOutput, "용인대 명품 태권도")
	assert.Contains(t, busToolOutput, "14:15")

	schoolTool := agent.NewSchoolTimetableTool(schoolSvc)
	schoolToolOutput, err := schoolTool.Execute(t.Context(), `{"day": "월요일"}`)
	require.NoError(t, err)
	assert.Contains(t, schoolToolOutput, "도덕")
	assert.Contains(t, schoolToolOutput, "사회")

	// Verify dynamic domain prompt
	domainPrompt := agent.BuildDomainKnowledge(busSvc, schoolSvc)
	assert.Contains(t, domainPrompt, "용인대 명품 태권도")
	assert.Contains(t, domainPrompt, "4학년 2반")

	// =========================================================================
	// Phase 5: Delete file via DELETE /api/data/files and verify hot-reload
	// =========================================================================
	delReq, _ := http.NewRequest(http.MethodDelete, baseURL+"/api/data/files?filename=태권도_1호차_등원.json", nil)
	delReq.Header.Set("X-Admin-Token", adminToken)
	delResp, err := http.DefaultClient.Do(delReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, delResp.StatusCode)
	delResp.Body.Close()

	// Verify file is physically removed from disk
	assert.NoFileExists(t, filepath.Join(tempDataDir, "태권도_1호차_등원.json"))

	// Verify catalog no longer contains 태권도
	catResp3 := getCatalogs(t, baseURL, adminToken)
	assert.NotContains(t, catResp3.Files, "태권도_1호차_등원.json")
	assert.Contains(t, catResp3.Files, "2026_4-2_school_timetable.json")

	// Re-querying 태권도 should no longer return the bus timetable
	deletedBusQueryResp := sendSkillMessage(t, baseURL, "태권도 우미린 2차 버스 몇 시에 와?")
	deletedBusText := getResponseText(deletedBusQueryResp)
	assert.NotContains(t, deletedBusText, "용인대 명품 태권도")
}

// Helpers

func contextWithTimeout(t *testing.T, d time.Duration) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(context.Background(), d)
}

func getCatalogs(t *testing.T, baseURL, token string) adminResponse {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, baseURL+"/api/data/catalogs", nil)
	require.NoError(t, err)
	req.Header.Set("X-Admin-Token", token)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	var cat adminResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&cat))
	return cat
}

func uploadFileMultipart(t *testing.T, baseURL, token, filename string, content []byte) *http.Response {
	t.Helper()
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	part, err := w.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/data/upload", &b)
	require.NoError(t, err)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("X-Admin-Token", token)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func uploadFileRawJSON(t *testing.T, baseURL, token, filename string, content []byte) *http.Response {
	t.Helper()
	url := fmt.Sprintf("%s/api/data/upload?filename=%s", baseURL, filename)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(content))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-Token", token)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func sendSkillMessage(t *testing.T, baseURL, utterance string) openbuilder.SkillResponse {
	t.Helper()
	payload := openbuilder.SkillPayload{
		UserRequest: openbuilder.UserRequest{
			Utterance: utterance,
			User: openbuilder.ChatUser{
				ID: "e2e-test-user-01",
			},
		},
	}
	body, _ := json.Marshal(payload)

	resp, err := http.Post(baseURL+"/skill", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	var skillResp openbuilder.SkillResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&skillResp))
	return skillResp
}

func getResponseText(resp openbuilder.SkillResponse) string {
	if len(resp.Template.Outputs) == 0 {
		return ""
	}
	output := resp.Template.Outputs[0]
	if output.SimpleText != nil {
		return output.SimpleText.Text
	}
	if output.BasicCard != nil {
		return output.BasicCard.Description
	}
	if output.TextCard != nil {
		return output.TextCard.Description
	}
	return ""
}
