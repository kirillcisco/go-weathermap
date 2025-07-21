package utils

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

func RespondWithError(w http.ResponseWriter, code int, message string) {
	RespondWithJSON(w, code, map[string]string{"error": message})
}

func RespondWithJSON(w http.ResponseWriter, code int, payload any) {
	response, _ := json.Marshal(payload)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(response)
}

func ParseBandwidth(bw string) int64 {
	if bw == "" {
		return 100_000_000 // by default return 100 MB for tests
	}
	bw = strings.ToUpper(strings.TrimSpace(bw))
	mult := int64(1_000_000)
	if strings.HasSuffix(bw, "G") {
		mult = 1_000_000_000
		bw = strings.TrimSuffix(bw, "G")
	} else if strings.HasSuffix(bw, "M") {
		mult = 1_000_000
		bw = strings.TrimSuffix(bw, "M")
	} else if strings.HasSuffix(bw, "K") {
		mult = 1_000
		bw = strings.TrimSuffix(bw, "K")
	}
	val, _ := strconv.ParseInt(bw, 10, 64)
	if val <= 0 {
		return 2_000_000
	}
	return val * mult / 8
}
