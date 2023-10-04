package stats

import (
	"sync"
	"time"
)

type responseState struct {
	responseTime time.Duration
	statusCode   int
	timeStamp    time.Time
}

type TargetStats struct {
	StatStartDate time.Time `json:"statStartDate"`
	// the total number of requests
	TotalRequestCount int `json:"totalRequestCount"`
	// average response time
	TotalAvgResponseTime time.Duration `json:"totalAvgResponseTime"`

	// the time window to base the stats on
	// e.g. 1 minute would capture the responses of the last minute to calculate the stats
	WindowDuration time.Duration `json:"windowDuration"`

	// the average response time in the window duration
	AvgResponseTime time.Duration `json:"avgResponseTime"`
	// the number of requests in the window duration
	RequestCount int `json:"requestCount"`
	// the number of requests per second in the window duration
	RequestRate float64 `json:"requestRate"`

	// the number of requests that failed (Status >= 400) in the window duration
	ErrorRate float64 `json:"errorRate"`
}

type StatRecorder struct {
	sync.Mutex
	firstRequest time.Time
	windowSize   time.Duration
	// the responses to base the stats on
	responseWindow []responseState

	// the total number of requests
	requestCount int
	// average response time
	avgResponseTime time.Duration
}

func newStatRecorder(windowSize time.Duration) *StatRecorder {
	return &StatRecorder{
		windowSize:     windowSize,
		responseWindow: make([]responseState, 0),
	}
}

func (t *StatRecorder) AddResponse(responseTime time.Duration, statusCode int) {
	t.Lock()
	defer t.Unlock()

	if t.firstRequest.IsZero() {
		t.firstRequest = time.Now()
	}

	t.requestCount++
	t.avgResponseTime = (t.avgResponseTime*time.Duration(t.requestCount-1) + responseTime) / time.Duration(t.requestCount)

	newWindow := make([]responseState, 0, len(t.responseWindow)+1)
	for _, state := range t.responseWindow {
		if time.Since(state.timeStamp) < t.windowSize {
			newWindow = append(newWindow, state)
		}
	}
	newWindow = append(newWindow, responseState{responseTime: responseTime, statusCode: statusCode, timeStamp: time.Now()})
	t.responseWindow = newWindow
}

func (t *StatRecorder) GetStat() TargetStats {
	t.Lock()
	defer t.Unlock()

	// update window
	newWindow := make([]responseState, 0, len(t.responseWindow)+1)
	for _, state := range t.responseWindow {
		if time.Since(state.timeStamp) < t.windowSize {
			newWindow = append(newWindow, state)
		}
	}
	t.responseWindow = newWindow

	// calculate stats
	return TargetStats{
		TotalRequestCount:    t.requestCount,
		TotalAvgResponseTime: t.avgResponseTime,
		WindowDuration:       t.windowSize,
		AvgResponseTime:      getAvgResponseTime(newWindow),
		RequestCount:         len(newWindow),
		RequestRate:          getRequestRate(newWindow),
		ErrorRate:            getErrorRate(newWindow),
		StatStartDate:        t.firstRequest,
	}
}

func getAvgResponseTime(stats []responseState) time.Duration {
	var totalResponseTime time.Duration
	for _, state := range stats {
		totalResponseTime += state.responseTime
	}
	if len(stats) == 0 {
		return 0
	}
	return totalResponseTime / time.Duration(len(stats))
}

func getRequestRate(stats []responseState) float64 {
	if len(stats) == 0 {
		return 0
	}

	firstRequest := stats[0].timeStamp
	lastRequest := stats[len(stats)-1].timeStamp
	return float64(len(stats)) / lastRequest.Sub(firstRequest).Seconds()
}

func getErrorRate(stats []responseState) float64 {
	if len(stats) == 0 {
		return 0
	}

	var errorCount int
	for _, state := range stats {
		if state.statusCode >= 400 {
			errorCount++
		}
	}

	return float64(errorCount) / float64(len(stats))
}
