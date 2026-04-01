package util

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
)

// APIResponse 统一API响应格式
type APIResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

func WriteJSON(w http.ResponseWriter, code int, msg string, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	resp := APIResponse{Code: code, Msg: msg, Data: data}
	json.NewEncoder(w).Encode(resp)
}

func Success(w http.ResponseWriter, data interface{}) {
	WriteJSON(w, 0, "ok", data)
}

func Error(w http.ResponseWriter, code int, msg string) {
	WriteJSON(w, code, msg, nil)
}

func ParseJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// CalcDistance 计算两点间距离（米），Haversine公式
func CalcDistance(lat1, lng1, lat2, lng2 float64) float64 {
	const R = 6371000
	rad := math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLng := (lng2 - lng1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*
			math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}

// FormatDistance 格式化距离文本
func FormatDistance(meters float64) string {
	if meters < 1000 {
		return fmt.Sprintf("%.0fm", meters)
	}
	return fmt.Sprintf("%.1fkm", meters/1000)
}
