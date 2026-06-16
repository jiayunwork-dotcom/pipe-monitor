package utils

import (
	"crypto/rand"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"
)

type JSONString string

func (j JSONString) Value() (driver.Value, error) {
	if j == "" {
		return "{}", nil
	}
	return string(j), nil
}

func (j *JSONString) Scan(value interface{}) error {
	if value == nil {
		*j = "{}"
		return nil
	}
	switch v := value.(type) {
	case string:
		if v == "" {
			*j = "{}"
		} else {
			*j = JSONString(v)
		}
	case []byte:
		if len(v) == 0 {
			*j = "{}"
		} else {
			*j = JSONString(string(v))
		}
	default:
		return fmt.Errorf("unsupported scan type for JSONString: %T", value)
	}
	return nil
}

func GenerateToken(n int) string {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func UintToStr(v uint) string {
	return strconv.FormatUint(uint64(v), 10)
}

func StrToUint(s string) uint {
	v, _ := strconv.ParseUint(s, 10, 64)
	return uint(v)
}

func ToJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func FromJSON(s string, v interface{}) error {
	return json.Unmarshal([]byte(s), v)
}

func IsWorkday(t time.Time) bool {
	wd := t.Weekday()
	return wd != time.Saturday && wd != time.Sunday
}

func ParseHM(s string) (hour, minute int, err error) {
	if len(s) != 5 {
		return 0, 0, fmt.Errorf("invalid time format, expected HH:MM")
	}
	_, err = fmt.Sscanf(s, "%d:%d", &hour, &minute)
	return
}

func GetTodayDeadline(t time.Time, hm string) (time.Time, error) {
	h, m, err := ParseHM(hm)
	if err != nil {
		return time.Time{}, err
	}
	loc := t.Location()
	return time.Date(t.Year(), t.Month(), t.Day(), h, m, 0, 0, loc), nil
}

func PercentileInt(values []int, p float64) int {
	if len(values) == 0 {
		return 0
	}
	sorted := make([]int, len(values))
	copy(sorted, values)
	sort.Ints(sorted)
	idx := int(math.Ceil(p/100.0*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func AverageInt(values []int) int {
	if len(values) == 0 {
		return 0
	}
	sum := 0
	for _, v := range values {
		sum += v
	}
	return sum / len(values)
}

func MaxInt(values []int) int {
	if len(values) == 0 {
		return 0
	}
	m := values[0]
	for _, v := range values[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

func RoundFloat(f float64, n int) float64 {
	multiplier := math.Pow10(n)
	return math.Round(f*multiplier) / multiplier
}
