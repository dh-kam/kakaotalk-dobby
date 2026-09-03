package skill

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/dh-kam/kakaotalk-dobby/pkg/academy"
	"github.com/dh-kam/kakaotalk-dobby/pkg/school"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestServer(t *testing.T, token string) (*httptest.Server, string, *academy.Service, *school.Service) {
	tempDir, err := os.MkdirTemp("", "kakaobot-admin-test-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	busSvc := academy.NewService()
	schoolSvc := school.NewService()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/data/upload", newAdminUploadHandler(tempDir, token, busSvc, schoolSvc))
	mux.HandleFunc("/api/data/reload", newAdminReloadHandler(tempDir, token, busSvc, schoolSvc))
	mux.HandleFunc("/api/data/catalogs", newAdminCatalogsHandler(tempDir, token, busSvc, schoolSvc))
	mux.HandleFunc("/api/data/files", newAdminDeleteHandler(tempDir, token, busSvc, schoolSvc))

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	return ts, tempDir, busSvc, schoolSvc
}

func TestAdminAPI_AuthEnforcement(t *testing.T) {
	ts, _, _, _ := setupTestServer(t, "super-secret-token")

	// 1. No token -> 401
	resp, err := http.Post(ts.URL+"/api/data/reload", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	// 2. Wrong token -> 401
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/data/reload", nil)
	req.Header.Set("X-Admin-Token", "wrong-token")
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	// 3. Valid X-Admin-Token -> 200
	req, _ = http.NewRequest(http.MethodPost, ts.URL+"/api/data/reload", nil)
	req.Header.Set("X-Admin-Token", "super-secret-token")
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// 4. Valid Authorization: Bearer -> 200
	req, _ = http.NewRequest(http.MethodPost, ts.URL+"/api/data/reload", nil)
	req.Header.Set("Authorization", "Bearer super-secret-token")
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAdminAPI_UploadBusSchedule_Multipart(t *testing.T) {
	ts, tempDir, busSvc, _ := setupTestServer(t, "test-token")

	busJSON := `{
		"academy": {
			"id": "taekwondo-1",
			"name": "용인대 태권도",
			"vehicle_number": "1호차",
			"type": "등원",
			"contact": "010-1234-5678"
		},
		"classes": [
			{"class_id": "c1", "class_name": "초등 1부", "class_time": "14:00"}
		],
		"stops": [
			{
				"sequence": 1,
				"location": "우미린 2차 정문",
				"display_schedules": {"초등 1부": "13:40"}
			}
		]
	}`

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "taekwondo.json")
	require.NoError(t, err)
	_, err = part.Write([]byte(busJSON))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/data/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Admin-Token", "test-token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var res adminResponse
	err = json.NewDecoder(resp.Body).Decode(&res)
	require.NoError(t, err)
	assert.Equal(t, "success", res.Status)
	assert.Equal(t, "academy_bus_schedule", res.DataType)
	assert.Equal(t, "taekwondo.json", res.Filename)

	// Verify file on disk
	savedPath := filepath.Join(tempDir, "taekwondo.json")
	assert.FileExists(t, savedPath)

	// Verify memory service reloaded
	academies := busSvc.ListAcademies()
	require.Len(t, academies, 1)
	assert.Contains(t, academies[0], "용인대 태권도")
}

func TestAdminAPI_UploadSchoolTimetable_RawJSON(t *testing.T) {
	ts, tempDir, _, schoolSvc := setupTestServer(t, "test-token")

	schoolJSON := `{
		"title": "2026학년도 5학년 3반 시간표",
		"school_year": 2026,
		"grade": 5,
		"class_number": 3,
		"weekly_timetable": {
			"monday": {
				"day_name": "월요일",
				"schedule": [
					{"period": 1, "time": "09:00~09:40", "subject": "국어"}
				]
			}
		}
	}`

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/data/upload?filename=2026_5-3_timetable.json", bytes.NewBufferString(schoolJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-Token", "test-token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var res adminResponse
	err = json.NewDecoder(resp.Body).Decode(&res)
	require.NoError(t, err)
	assert.Equal(t, "success", res.Status)
	assert.Equal(t, "school_timetable", res.DataType)

	// Verify file on disk
	savedPath := filepath.Join(tempDir, "2026_5-3_timetable.json")
	assert.FileExists(t, savedPath)

	// Verify school service reloaded
	summary := schoolSvc.GetSummary()
	assert.Contains(t, summary, "5학년 3반")
}

func TestAdminAPI_UploadInvalidSchema(t *testing.T) {
	ts, _, _, _ := setupTestServer(t, "test-token")

	invalidJSON := `{"name": "unknown_data", "version": 1}`

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/data/upload?filename=invalid.json", bytes.NewBufferString(invalidJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-Token", "test-token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var res adminResponse
	err = json.NewDecoder(resp.Body).Decode(&res)
	require.NoError(t, err)
	assert.Equal(t, "error", res.Status)
	assert.Contains(t, res.Error, "Invalid schema")
}

func TestAdminAPI_PathTraversalDefense(t *testing.T) {
	ts, tempDir, _, _ := setupTestServer(t, "test-token")

	schoolJSON := `{
		"title": "2026학년도 1학년 1반 시간표",
		"school_year": 2026,
		"grade": 1,
		"class_number": 1,
		"weekly_timetable": {
			"monday": {"day_name": "월요일", "schedule": []}
		}
	}`

	// Malicious path traversal
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/data/upload?filename=../../../../tmp/hacked.json", bytes.NewBufferString(schoolJSON))
	req.Header.Set("X-Admin-Token", "test-token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Should be saved safely under tempDir/hacked.json, not /tmp/hacked.json
	assert.FileExists(t, filepath.Join(tempDir, "hacked.json"))
}

func TestAdminAPI_CatalogsAndDeletion(t *testing.T) {
	ts, tempDir, busSvc, _ := setupTestServer(t, "test-token")

	// 1. Create a dummy file in tempDir
	testFile := filepath.Join(tempDir, "sample_bus.json")
	busJSON := `{
		"academy": {"id": "test", "name": "피아노 학원", "vehicle_number": "1호차", "type": "등원"},
		"stops": [{"sequence": 1, "location": "상가 앞"}]
	}`
	require.NoError(t, os.WriteFile(testFile, []byte(busJSON), 0644))
	require.NoError(t, busSvc.LoadFromDir(tempDir))

	// 2. Query catalogs
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/data/catalogs", nil)
	req.Header.Set("X-Admin-Token", "test-token")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	var catRes adminResponse
	err = json.NewDecoder(resp.Body).Decode(&catRes)
	require.NoError(t, err)
	assert.Equal(t, "success", catRes.Status)
	assert.Contains(t, catRes.Files, "sample_bus.json")
	assert.Equal(t, 1, int(catRes.Catalogs["academyRoutesCount"].(float64)))

	// 3. Delete file
	delReq, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/data/files?filename=sample_bus.json", nil)
	delReq.Header.Set("X-Admin-Token", "test-token")
	delResp, err := http.DefaultClient.Do(delReq)
	require.NoError(t, err)
	defer delResp.Body.Close()

	assert.Equal(t, http.StatusOK, delResp.StatusCode)
	assert.NoFileExists(t, testFile)
	assert.Empty(t, busSvc.ListAcademies())
}
