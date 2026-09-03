package skill

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/dh-kam/kakaotalk-dobby/pkg/academy"
	"github.com/dh-kam/kakaotalk-dobby/pkg/school"
)

type adminResponse struct {
	Status   string         `json:"status"`
	Message  string         `json:"message,omitempty"`
	Error    string         `json:"error,omitempty"`
	Filename string         `json:"filename,omitempty"`
	DataType string         `json:"data_type,omitempty"`
	DataDir  string         `json:"data_dir,omitempty"`
	Files    []string       `json:"files,omitempty"`
	Catalogs map[string]any `json:"catalogs,omitempty"`
}

func checkAdminAuth(r *http.Request, token string) bool {
	if token == "" {
		// When no token is configured, allow requests (open/dev mode)
		return true
	}
	if r.Header.Get("X-Admin-Token") == token {
		return true
	}
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") && strings.TrimPrefix(authHeader, "Bearer ") == token {
		return true
	}
	return false
}

func writeAdminJSON(w http.ResponseWriter, status int, resp adminResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

func buildCatalogsMap(busSvc *academy.Service, schoolSvc *school.Service) map[string]any {
	catalogs := make(map[string]any)
	if busSvc != nil {
		academies := busSvc.ListAcademies()
		catalogs["academyRoutesCount"] = len(academies)
		catalogs["academyRoutes"] = academies
	}
	if schoolSvc != nil {
		catalogs["schoolTimetables"] = schoolSvc.GetSummary()
	}
	return catalogs
}

// newAdminUploadHandler handles POST /api/data/upload.
func newAdminUploadHandler(dataDir, adminToken string, busSvc *academy.Service, schoolSvc *school.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeAdminJSON(w, http.StatusMethodNotAllowed, adminResponse{
				Status: "error",
				Error:  "Method not allowed: use POST",
			})
			return
		}

		if !checkAdminAuth(r, adminToken) {
			writeAdminJSON(w, http.StatusUnauthorized, adminResponse{
				Status: "error",
				Error:  "Unauthorized: valid X-Admin-Token or Authorization: Bearer token required",
			})
			return
		}

		// Limit upload to 10MB
		r.Body = http.MaxBytesReader(w, r.Body, 10<<20)

		var filename string
		var fileBytes []byte

		contentType := r.Header.Get("Content-Type")
		if strings.HasPrefix(contentType, "multipart/form-data") {
			if err := r.ParseMultipartForm(10 << 20); err != nil {
				writeAdminJSON(w, http.StatusBadRequest, adminResponse{
					Status: "error",
					Error:  fmt.Sprintf("Failed to parse multipart form: %v", err),
				})
				return
			}
			file, header, err := r.FormFile("file")
			if err != nil {
				writeAdminJSON(w, http.StatusBadRequest, adminResponse{
					Status: "error",
					Error:  "Missing 'file' field in multipart form-data",
				})
				return
			}
			defer file.Close()

			filename = header.Filename
			if formFilename := r.FormValue("filename"); formFilename != "" {
				filename = formFilename
			}

			data, err := io.ReadAll(file)
			if err != nil {
				writeAdminJSON(w, http.StatusBadRequest, adminResponse{
					Status: "error",
					Error:  fmt.Sprintf("Failed to read uploaded file: %v", err),
				})
				return
			}
			fileBytes = data
		} else {
			// Direct JSON payload
			filename = r.URL.Query().Get("filename")
			if filename == "" {
				filename = r.Header.Get("X-File-Name")
			}
			if filename == "" {
				writeAdminJSON(w, http.StatusBadRequest, adminResponse{
					Status: "error",
					Error:  "Filename required: specify via ?filename=... query parameter or X-File-Name header",
				})
				return
			}

			data, err := io.ReadAll(r.Body)
			if err != nil {
				writeAdminJSON(w, http.StatusBadRequest, adminResponse{
					Status: "error",
					Error:  fmt.Sprintf("Failed to read body: %v", err),
				})
				return
			}
			defer r.Body.Close()
			fileBytes = data
		}

		// Prevent path traversal
		cleanName := filepath.Base(filename)
		if !strings.HasSuffix(strings.ToLower(cleanName), ".json") {
			writeAdminJSON(w, http.StatusBadRequest, adminResponse{
				Status: "error",
				Error:  fmt.Sprintf("Invalid file extension for %q: must end with .json", cleanName),
			})
			return
		}

		// Schema validation
		dataType := ""
		var busSched academy.BusSchedule
		if err := json.Unmarshal(fileBytes, &busSched); err == nil && busSched.Academy.Name != "" && (len(busSched.Stops) > 0 || len(busSched.Classes) > 0) {
			dataType = "academy_bus_schedule"
		} else {
			var schoolTT school.Timetable
			if err := json.Unmarshal(fileBytes, &schoolTT); err == nil && schoolTT.Grade > 0 && schoolTT.ClassNumber > 0 && len(schoolTT.WeeklyTimetable) > 0 {
				dataType = "school_timetable"
			}
		}

		if dataType == "" {
			writeAdminJSON(w, http.StatusBadRequest, adminResponse{
				Status: "error",
				Error:  "Invalid schema: JSON must conform to either Academy BusSchedule (academy.name and stops) or School Timetable (grade, class_number, weekly_timetable)",
			})
			return
		}

		// Ensure target directory exists
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			writeAdminJSON(w, http.StatusInternalServerError, adminResponse{
				Status: "error",
				Error:  fmt.Sprintf("Failed to create data directory: %v", err),
			})
			return
		}

		targetPath := filepath.Join(dataDir, cleanName)
		if err := os.WriteFile(targetPath, fileBytes, 0644); err != nil {
			writeAdminJSON(w, http.StatusInternalServerError, adminResponse{
				Status: "error",
				Error:  fmt.Sprintf("Failed to write file to disk: %v", err),
			})
			return
		}

		// Hot-reload memory services
		if busSvc != nil {
			_ = busSvc.ReloadFromDir(dataDir)
		}
		if schoolSvc != nil {
			_ = schoolSvc.ReloadFromDir(dataDir)
		}

		writeAdminJSON(w, http.StatusOK, adminResponse{
			Status:   "success",
			Message:  fmt.Sprintf("Successfully saved %q and reloaded services in-memory", cleanName),
			Filename: cleanName,
			DataType: dataType,
			DataDir:  dataDir,
			Catalogs: buildCatalogsMap(busSvc, schoolSvc),
		})
	}
}

// newAdminReloadHandler handles POST /api/data/reload.
func newAdminReloadHandler(dataDir, adminToken string, busSvc *academy.Service, schoolSvc *school.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeAdminJSON(w, http.StatusMethodNotAllowed, adminResponse{
				Status: "error",
				Error:  "Method not allowed: use POST",
			})
			return
		}

		if !checkAdminAuth(r, adminToken) {
			writeAdminJSON(w, http.StatusUnauthorized, adminResponse{
				Status: "error",
				Error:  "Unauthorized: valid X-Admin-Token or Authorization: Bearer token required",
			})
			return
		}

		if busSvc != nil {
			if err := busSvc.ReloadFromDir(dataDir); err != nil {
				writeAdminJSON(w, http.StatusInternalServerError, adminResponse{
					Status: "error",
					Error:  fmt.Sprintf("Failed to reload bus schedules: %v", err),
				})
				return
			}
		}

		if schoolSvc != nil {
			if err := schoolSvc.ReloadFromDir(dataDir); err != nil {
				writeAdminJSON(w, http.StatusInternalServerError, adminResponse{
					Status: "error",
					Error:  fmt.Sprintf("Failed to reload school timetables: %v", err),
				})
				return
			}
		}

		writeAdminJSON(w, http.StatusOK, adminResponse{
			Status:   "success",
			Message:  "Successfully reloaded all data from directory",
			DataDir:  dataDir,
			Catalogs: buildCatalogsMap(busSvc, schoolSvc),
		})
	}
}

// newAdminCatalogsHandler handles GET /api/data/catalogs.
func newAdminCatalogsHandler(dataDir, adminToken string, busSvc *academy.Service, schoolSvc *school.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeAdminJSON(w, http.StatusMethodNotAllowed, adminResponse{
				Status: "error",
				Error:  "Method not allowed: use GET",
			})
			return
		}

		if !checkAdminAuth(r, adminToken) {
			writeAdminJSON(w, http.StatusUnauthorized, adminResponse{
				Status: "error",
				Error:  "Unauthorized: valid X-Admin-Token or Authorization: Bearer token required",
			})
			return
		}

		var files []string
		entries, err := os.ReadDir(dataDir)
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
					files = append(files, entry.Name())
				}
			}
		}

		writeAdminJSON(w, http.StatusOK, adminResponse{
			Status:   "success",
			DataDir:  dataDir,
			Files:    files,
			Catalogs: buildCatalogsMap(busSvc, schoolSvc),
		})
	}
}

// newAdminDeleteHandler handles DELETE /api/data/files.
func newAdminDeleteHandler(dataDir, adminToken string, busSvc *academy.Service, schoolSvc *school.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			writeAdminJSON(w, http.StatusMethodNotAllowed, adminResponse{
				Status: "error",
				Error:  "Method not allowed: use DELETE",
			})
			return
		}

		if !checkAdminAuth(r, adminToken) {
			writeAdminJSON(w, http.StatusUnauthorized, adminResponse{
				Status: "error",
				Error:  "Unauthorized: valid X-Admin-Token or Authorization: Bearer token required",
			})
			return
		}

		filename := r.URL.Query().Get("filename")
		if filename == "" {
			writeAdminJSON(w, http.StatusBadRequest, adminResponse{
				Status: "error",
				Error:  "Filename required: specify via ?filename=... query parameter",
			})
			return
		}

		cleanName := filepath.Base(filename)
		targetPath := filepath.Join(dataDir, cleanName)

		if err := os.Remove(targetPath); err != nil {
			writeAdminJSON(w, http.StatusNotFound, adminResponse{
				Status: "error",
				Error:  fmt.Sprintf("Failed to delete file %q: %v", cleanName, err),
			})
			return
		}

		if busSvc != nil {
			_ = busSvc.ReloadFromDir(dataDir)
		}
		if schoolSvc != nil {
			_ = schoolSvc.ReloadFromDir(dataDir)
		}

		writeAdminJSON(w, http.StatusOK, adminResponse{
			Status:   "success",
			Message:  fmt.Sprintf("Successfully deleted file %q and reloaded services", cleanName),
			Filename: cleanName,
			DataDir:  dataDir,
			Catalogs: buildCatalogsMap(busSvc, schoolSvc),
		})
	}
}
