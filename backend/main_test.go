package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ─── Helpers ───────────────────────────────────────────────────────────────────

func floatPtr(f float64) *float64 { return &f }

func approxEqual(a, b float64, tol float64) bool {
	return math.Abs(a-b) < tol
}

// ─── calculateAnnualAvg Tests ──────────────────────────────────────────────────

func TestCalculateAnnualAvg_EmptyInput(t *testing.T) {
	result := calculateAnnualAvg(nil)
	if len(result) != 0 {
		t.Errorf("expected empty result for nil input, got %d items", len(result))
	}

	result = calculateAnnualAvg([]RawStationData{})
	if len(result) != 0 {
		t.Errorf("expected empty result for empty input, got %d items", len(result))
	}
}

func TestCalculateAnnualAvg_SingleYear_TMinAndTMax(t *testing.T) {
	raw := []RawStationData{
		{Date: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), ElementType: "TMIN", Value: 100},
		{Date: time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC), ElementType: "TMIN", Value: 200},
		{Date: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), ElementType: "TMAX", Value: 300},
		{Date: time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC), ElementType: "TMAX", Value: 400},
	}

	result := calculateAnnualAvg(raw)
	if len(result) != 1 {
		t.Fatalf("expected 1 year, got %d", len(result))
	}

	r := result[0]
	if r.Year != 2020 {
		t.Errorf("expected year 2020, got %d", r.Year)
	}
	// TMIN avg: (100+200)/2 = 150 -> 150/10 = 15.0
	if r.TMin == nil {
		t.Fatal("expected TMin to be non-nil")
	}
	if !approxEqual(*r.TMin, 15.0, 0.01) {
		t.Errorf("expected TMin ~15.0, got %f", *r.TMin)
	}
	// TMAX avg: (300+400)/2 = 350 -> 350/10 = 35.0
	if r.TMax == nil {
		t.Fatal("expected TMax to be non-nil")
	}
	if !approxEqual(*r.TMax, 35.0, 0.01) {
		t.Errorf("expected TMax ~35.0, got %f", *r.TMax)
	}
}

func TestCalculateAnnualAvg_MultipleYears_SortedByYear(t *testing.T) {
	raw := []RawStationData{
		{Date: time.Date(2022, 6, 15, 0, 0, 0, 0, time.UTC), ElementType: "TMIN", Value: 500},
		{Date: time.Date(2020, 3, 10, 0, 0, 0, 0, time.UTC), ElementType: "TMIN", Value: 100},
		{Date: time.Date(2021, 9, 20, 0, 0, 0, 0, time.UTC), ElementType: "TMIN", Value: 300},
	}

	result := calculateAnnualAvg(raw)
	if len(result) != 3 {
		t.Fatalf("expected 3 years, got %d", len(result))
	}

	// Should be sorted by year ascending
	if result[0].Year != 2020 || result[1].Year != 2021 || result[2].Year != 2022 {
		t.Errorf("expected years [2020,2021,2022], got [%d,%d,%d]",
			result[0].Year, result[1].Year, result[2].Year)
	}
}

func TestCalculateAnnualAvg_OnlyTMin_TMaxIsNil(t *testing.T) {
	raw := []RawStationData{
		{Date: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), ElementType: "TMIN", Value: 100},
	}

	result := calculateAnnualAvg(raw)
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if result[0].TMin == nil {
		t.Error("expected TMin to be set")
	}
	if result[0].TMax != nil {
		t.Error("expected TMax to be nil when no TMAX data provided")
	}
}

func TestCalculateAnnualAvg_OnlyTMax_TMinIsNil(t *testing.T) {
	raw := []RawStationData{
		{Date: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), ElementType: "TMAX", Value: 250},
	}

	result := calculateAnnualAvg(raw)
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if result[0].TMax == nil {
		t.Error("expected TMax to be set")
	}
	if result[0].TMin != nil {
		t.Error("expected TMin to be nil when no TMIN data provided")
	}
}

func TestCalculateAnnualAvg_DividesByTen(t *testing.T) {
	// NOAA values are in tenths of degrees
	raw := []RawStationData{
		{Date: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), ElementType: "TMIN", Value: 157},
	}

	result := calculateAnnualAvg(raw)
	// 157 / 10 = 15.7
	if !approxEqual(*result[0].TMin, 15.7, 0.01) {
		t.Errorf("expected TMin ~15.7 (value/10), got %f", *result[0].TMin)
	}
}

func TestCalculateAnnualAvg_RoundsToTwoDecimals(t *testing.T) {
	raw := []RawStationData{
		{Date: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), ElementType: "TMIN", Value: 100},
		{Date: time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC), ElementType: "TMIN", Value: 200},
		{Date: time.Date(2020, 1, 3, 0, 0, 0, 0, time.UTC), ElementType: "TMIN", Value: 103},
	}
	result := calculateAnnualAvg(raw)
	// avg = (100+200+103)/3 = 134.333... -> /10 = 13.4333... -> rounded = 13.43
	if !approxEqual(*result[0].TMin, 13.43, 0.001) {
		t.Errorf("expected TMin ~13.43 (rounded to 2 decimals), got %f", *result[0].TMin)
	}
}

func TestCalculateAnnualAvg_NegativeValues(t *testing.T) {
	raw := []RawStationData{
		{Date: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), ElementType: "TMIN", Value: -200},
		{Date: time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC), ElementType: "TMIN", Value: -100},
	}
	result := calculateAnnualAvg(raw)
	// avg = (-200 + -100)/2 = -150 -> /10 = -15.0
	if !approxEqual(*result[0].TMin, -15.0, 0.01) {
		t.Errorf("expected TMin ~-15.0, got %f", *result[0].TMin)
	}
}

// ─── calculateSeasonalAvg Tests ────────────────────────────────────────────────

func TestCalculateSeasonalAvg_EmptyInput(t *testing.T) {
	result := calculateSeasonalAvg(nil)
	if len(result) != 0 {
		t.Errorf("expected empty result for nil input, got %d", len(result))
	}
}

func TestCalculateSeasonalAvg_CorrectSeasonAssignment(t *testing.T) {
	// One data point per month, test which season each falls into
	months := []struct {
		month  time.Month
		season string
	}{
		{time.January, "Winter"},
		{time.February, "Winter"},
		{time.March, "Spring"},
		{time.April, "Spring"},
		{time.May, "Spring"},
		{time.June, "Summer"},
		{time.July, "Summer"},
		{time.August, "Summer"},
		{time.September, "Autumn"},
		{time.October, "Autumn"},
		{time.November, "Autumn"},
		{time.December, "Winter"},
	}

	for _, tc := range months {
		t.Run(tc.month.String(), func(t *testing.T) {
			raw := []RawStationData{
				{Date: time.Date(2020, tc.month, 15, 0, 0, 0, 0, time.UTC), ElementType: "TMIN", Value: 100},
			}
			result := calculateSeasonalAvg(raw)
			if len(result) != 1 {
				t.Fatalf("expected 1 result, got %d", len(result))
			}
			if result[0].Season != tc.season {
				t.Errorf("month %s: expected season %q, got %q", tc.month, tc.season, result[0].Season)
			}
		})
	}
}

func TestCalculateSeasonalAvg_AggregatesWithinSeason(t *testing.T) {
	raw := []RawStationData{
		// Summer 2020: Jun, Jul, Aug TMIN values
		{Date: time.Date(2020, 6, 15, 0, 0, 0, 0, time.UTC), ElementType: "TMIN", Value: 200},
		{Date: time.Date(2020, 7, 15, 0, 0, 0, 0, time.UTC), ElementType: "TMIN", Value: 250},
		{Date: time.Date(2020, 8, 15, 0, 0, 0, 0, time.UTC), ElementType: "TMIN", Value: 300},
	}
	result := calculateSeasonalAvg(raw)
	if len(result) != 1 {
		t.Fatalf("expected 1 seasonal result, got %d", len(result))
	}
	// avg = (200+250+300)/3 = 250 -> /10 = 25.0
	if result[0].Season != "Summer" {
		t.Errorf("expected Summer, got %s", result[0].Season)
	}
	if !approxEqual(*result[0].TMin, 25.0, 0.01) {
		t.Errorf("expected TMin ~25.0, got %f", *result[0].TMin)
	}
}

func TestCalculateSeasonalAvg_SortOrder(t *testing.T) {
	// Create data for all 4 seasons in 2020, inserted out of order
	raw := []RawStationData{
		{Date: time.Date(2020, 7, 15, 0, 0, 0, 0, time.UTC), ElementType: "TMIN", Value: 100},  // Summer
		{Date: time.Date(2020, 1, 15, 0, 0, 0, 0, time.UTC), ElementType: "TMIN", Value: 100},  // Winter
		{Date: time.Date(2020, 10, 15, 0, 0, 0, 0, time.UTC), ElementType: "TMIN", Value: 100}, // Autumn
		{Date: time.Date(2020, 4, 15, 0, 0, 0, 0, time.UTC), ElementType: "TMIN", Value: 100},  // Spring
	}
	result := calculateSeasonalAvg(raw)
	if len(result) != 4 {
		t.Fatalf("expected 4 results, got %d", len(result))
	}
	expectedOrder := []string{"Winter", "Spring", "Summer", "Autumn"}
	for i, expected := range expectedOrder {
		if result[i].Season != expected {
			t.Errorf("position %d: expected %s, got %s", i, expected, result[i].Season)
		}
	}
}

func TestCalculateSeasonalAvg_MultipleYears_SortedByYearThenSeason(t *testing.T) {
	raw := []RawStationData{
		{Date: time.Date(2021, 7, 15, 0, 0, 0, 0, time.UTC), ElementType: "TMIN", Value: 100}, // Summer 2021
		{Date: time.Date(2020, 1, 15, 0, 0, 0, 0, time.UTC), ElementType: "TMIN", Value: 100}, // Winter 2020
		{Date: time.Date(2020, 7, 15, 0, 0, 0, 0, time.UTC), ElementType: "TMIN", Value: 100}, // Summer 2020
	}
	result := calculateSeasonalAvg(raw)
	if len(result) != 3 {
		t.Fatalf("expected 3 results, got %d", len(result))
	}
	// Expected: Winter 2020, Summer 2020, Summer 2021
	if result[0].Year != 2020 || result[0].Season != "Winter" {
		t.Errorf("pos 0: expected 2020 Winter, got %d %s", result[0].Year, result[0].Season)
	}
	if result[1].Year != 2020 || result[1].Season != "Summer" {
		t.Errorf("pos 1: expected 2020 Summer, got %d %s", result[1].Year, result[1].Season)
	}
	if result[2].Year != 2021 || result[2].Season != "Summer" {
		t.Errorf("pos 2: expected 2021 Summer, got %d %s", result[2].Year, result[2].Season)
	}
}

func TestCalculateSeasonalAvg_TMinAndTMaxBothPresent(t *testing.T) {
	raw := []RawStationData{
		{Date: time.Date(2020, 7, 1, 0, 0, 0, 0, time.UTC), ElementType: "TMIN", Value: 200},
		{Date: time.Date(2020, 7, 1, 0, 0, 0, 0, time.UTC), ElementType: "TMAX", Value: 350},
	}
	result := calculateSeasonalAvg(raw)
	if len(result) != 1 {
		t.Fatalf("expected 1, got %d", len(result))
	}
	if result[0].TMin == nil || result[0].TMax == nil {
		t.Fatal("expected both TMin and TMax to be non-nil")
	}
	if !approxEqual(*result[0].TMin, 20.0, 0.01) {
		t.Errorf("expected TMin ~20.0, got %f", *result[0].TMin)
	}
	if !approxEqual(*result[0].TMax, 35.0, 0.01) {
		t.Errorf("expected TMax ~35.0, got %f", *result[0].TMax)
	}
}

func TestCalculateSeasonalAvg_NilPointersWhenMissing(t *testing.T) {
	raw := []RawStationData{
		{Date: time.Date(2020, 7, 1, 0, 0, 0, 0, time.UTC), ElementType: "TMIN", Value: 200},
	}
	result := calculateSeasonalAvg(raw)
	if result[0].TMin == nil {
		t.Error("expected TMin non-nil")
	}
	if result[0].TMax != nil {
		t.Error("expected TMax nil when no TMAX data")
	}
}

// ─── findStations Tests ────────────────────────────────────────────────────────

// setupGlobalState sets up the global allStations and inventoryMap for testing.
// Must be called before findStations tests. Cleans up after test completes.
func setupGlobalState(t *testing.T, stations []*Station, inventory map[string]*StationInventory) {
	oldStations := allStations
	oldInventory := inventoryMap
	allStations = stations
	inventoryMap = inventory
	t.Cleanup(func() {
		allStations = oldStations
		inventoryMap = oldInventory
	})
}

func TestFindStations_EmptyStations(t *testing.T) {
	setupGlobalState(t, []*Station{}, map[string]*StationInventory{})
	result, err := findStations(52.5, 13.4, 100, 10, 1900, 2020)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 stations, got %d", len(result))
	}
}

func TestFindStations_FiltersOutOfRadius(t *testing.T) {
	lat1, long1 := 52.52, 13.405   // Berlin
	lat2, long2 := 48.8566, 2.3522 // Paris (~878 km from Berlin)

	setupGlobalState(t,
		[]*Station{
			{ID: "STN001", Name: "Berlin", Latitude: &lat1, Longitude: &long1},
			{ID: "STN002", Name: "Paris", Latitude: &lat2, Longitude: &long2},
		},
		map[string]*StationInventory{
			"STN001": {FirstYear: 1900, LastYear: 2023},
			"STN002": {FirstYear: 1900, LastYear: 2023},
		},
	)

	// Radius 100 km from Berlin - should only find Berlin
	result, err := findStations(52.52, 13.405, 100, 10, 1950, 2020)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 station within 100 km, got %d", len(result))
	}
	if result[0].ID != "STN001" {
		t.Errorf("expected STN001 (Berlin), got %s", result[0].ID)
	}

	// Radius 1000 km - should find both
	result, err = findStations(52.52, 13.405, 1000, 10, 1950, 2020)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 stations within 1000 km, got %d", len(result))
	}
}

func TestFindStations_FiltersbyInventoryYears(t *testing.T) {
	lat, long := 52.52, 13.405

	setupGlobalState(t,
		[]*Station{
			{ID: "STN001", Name: "Has Data", Latitude: &lat, Longitude: &long},
			{ID: "STN002", Name: "No Data", Latitude: &lat, Longitude: &long},
			{ID: "STN003", Name: "Partial", Latitude: &lat, Longitude: &long},
		},
		map[string]*StationInventory{
			"STN001": {FirstYear: 1900, LastYear: 2023},
			"STN002": {FirstYear: 2000, LastYear: 2010}, // firstYear > startYear requested
			// STN003 has no inventory entry at all
		},
	)

	result, err := findStations(52.52, 13.405, 100, 10, 1950, 2020)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 station, got %d", len(result))
	}
	if result[0].ID != "STN001" {
		t.Errorf("expected STN001, got %s", result[0].ID)
	}
}

func TestFindStations_SortsByDistance(t *testing.T) {
	latBerlin, longBerlin := 52.52, 13.405
	latPotsdam, longPotsdam := 52.3906, 13.0645 // ~27 km from Berlin center
	latDresden, longDresden := 51.0504, 13.7373 // ~166 km from Berlin

	setupGlobalState(t,
		[]*Station{
			{ID: "DRESDEN", Name: "Dresden", Latitude: &latDresden, Longitude: &longDresden},
			{ID: "POTSDAM", Name: "Potsdam", Latitude: &latPotsdam, Longitude: &longPotsdam},
			{ID: "BERLIN", Name: "Berlin Ctr", Latitude: &latBerlin, Longitude: &longBerlin},
		},
		map[string]*StationInventory{
			"DRESDEN": {FirstYear: 1900, LastYear: 2023},
			"POTSDAM": {FirstYear: 1900, LastYear: 2023},
			"BERLIN":  {FirstYear: 1900, LastYear: 2023},
		},
	)

	result, err := findStations(52.52, 13.405, 500, 10, 1950, 2020)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("expected 3, got %d", len(result))
	}

	// Berlin should be closest (essentially 0 distance), then Potsdam, then Dresden
	if result[0].ID != "BERLIN" {
		t.Errorf("expected BERLIN first (closest), got %s", result[0].ID)
	}
	if result[1].ID != "POTSDAM" {
		t.Errorf("expected POTSDAM second, got %s", result[1].ID)
	}
	if result[2].ID != "DRESDEN" {
		t.Errorf("expected DRESDEN third, got %s", result[2].ID)
	}

	// Verify distances are ascending
	if result[0].Distance > result[1].Distance || result[1].Distance > result[2].Distance {
		t.Error("stations not sorted by ascending distance")
	}
}

func TestFindStations_LimitResults(t *testing.T) {
	stations := make([]*Station, 20)
	inv := make(map[string]*StationInventory)
	lat, long := 52.52, 13.405
	for i := 0; i < 20; i++ {
		id := fmt.Sprintf("STN%03d", i)
		sLat := lat + float64(i)*0.01
		sLong := long
		stations[i] = &Station{ID: id, Name: id, Latitude: &sLat, Longitude: &sLong}
		inv[id] = &StationInventory{FirstYear: 1900, LastYear: 2023}
	}

	setupGlobalState(t, stations, inv)

	result, err := findStations(52.52, 13.405, 5000, 5, 1950, 2020)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 5 {
		t.Errorf("expected limit of 5, got %d", len(result))
	}
}

func TestFindStations_SkipsNilLatLong(t *testing.T) {
	lat := 52.52
	setupGlobalState(t,
		[]*Station{
			{ID: "STN001", Name: "No Lat", Latitude: nil, Longitude: &lat},
			{ID: "STN002", Name: "No Long", Latitude: &lat, Longitude: nil},
		},
		map[string]*StationInventory{
			"STN001": {FirstYear: 1900, LastYear: 2023},
			"STN002": {FirstYear: 1900, LastYear: 2023},
		},
	)

	result, err := findStations(52.52, 13.405, 5000, 10, 1950, 2020)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 stations (nil coords), got %d", len(result))
	}
}

func TestFindStations_HaversineDistance_Accuracy(t *testing.T) {
	// Known distance: Berlin (52.52, 13.405) to Munich (48.1351, 11.5820) ~504 km
	latBerlin, longBerlin := 52.52, 13.405
	latMunich, longMunich := 48.1351, 11.5820

	setupGlobalState(t,
		[]*Station{
			{ID: "MUNICH", Name: "Munich", Latitude: &latMunich, Longitude: &longMunich},
		},
		map[string]*StationInventory{
			"MUNICH": {FirstYear: 1900, LastYear: 2023},
		},
	)

	result, err := findStations(latBerlin, longBerlin, 600, 10, 1950, 2020)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 station, got %d", len(result))
	}

	// Berlin to Munich is approximately 504 km
	if result[0].Distance < 480 || result[0].Distance > 530 {
		t.Errorf("expected distance ~504 km, got %.1f km", result[0].Distance)
	}
}

func TestFindStations_LimitGreaterThanResults(t *testing.T) {
	lat, long := 52.52, 13.405
	setupGlobalState(t,
		[]*Station{
			{ID: "STN001", Name: "Only One", Latitude: &lat, Longitude: &long},
		},
		map[string]*StationInventory{
			"STN001": {FirstYear: 1900, LastYear: 2023},
		},
	)

	result, err := findStations(52.52, 13.405, 100, 100, 1950, 2020)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 station (fewer than limit), got %d", len(result))
	}
}

// ─── statusHandler Tests ───────────────────────────────────────────────────────

func TestStatusHandler_ReturnsOK(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	rec := httptest.NewRecorder()

	statusHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if body != "OK\n" {
		t.Errorf("expected body 'OK\\n', got %q", body)
	}
}

// ─── stationsHandler Tests ─────────────────────────────────────────────────────

func TestStationsHandler_MissingParams(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		errorMsg string
	}{
		{"missing lat", "?long=13&radius=100&limit=10&start=1950&end=2020", "latitude"},
		{"missing long", "?lat=52&radius=100&limit=10&start=1950&end=2020", "longitude"},
		{"missing radius", "?lat=52&long=13&limit=10&start=1950&end=2020", "radius"},
		{"missing limit", "?lat=52&long=13&radius=100&start=1950&end=2020", "limit"},
		{"missing start", "?lat=52&long=13&radius=100&limit=10&end=2020", "start year"},
		{"missing end", "?lat=52&long=13&radius=100&limit=10&start=1950", "end year"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/stations"+tc.query, nil)
			rec := httptest.NewRecorder()

			stationsHandler(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d", rec.Code)
			}

			var resp Response
			json.NewDecoder(rec.Body).Decode(&resp)
			if resp.ErrorMsg == "" {
				t.Error("expected error message, got empty")
			}
		})
	}
}

func TestStationsHandler_InvalidNumericParams(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{"invalid lat", "?lat=abc&long=13&radius=100&limit=10&start=1950&end=2020"},
		{"invalid long", "?lat=52&long=abc&radius=100&limit=10&start=1950&end=2020"},
		{"invalid radius", "?lat=52&long=13&radius=abc&limit=10&start=1950&end=2020"},
		{"invalid limit", "?lat=52&long=13&radius=100&limit=abc&start=1950&end=2020"},
		{"invalid start", "?lat=52&long=13&radius=100&limit=10&start=abc&end=2020"},
		{"invalid end", "?lat=52&long=13&radius=100&limit=10&start=1950&end=abc"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/stations"+tc.query, nil)
			rec := httptest.NewRecorder()

			stationsHandler(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d", rec.Code)
			}
		})
	}
}

func TestStationsHandler_ValidRequest(t *testing.T) {
	lat, long := 52.52, 13.405
	setupGlobalState(t,
		[]*Station{
			{ID: "STN001", Name: "Berlin", Latitude: &lat, Longitude: &long},
		},
		map[string]*StationInventory{
			"STN001": {FirstYear: 1900, LastYear: 2023},
		},
	)

	req := httptest.NewRequest(http.MethodGet, "/stations?lat=52.52&long=13.405&radius=100&limit=10&start=1950&end=2020", nil)
	rec := httptest.NewRecorder()

	stationsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp Response
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.ErrorMsg != "" {
		t.Errorf("expected no error, got %q", resp.ErrorMsg)
	}
	if resp.Data == nil {
		t.Error("expected non-nil data")
	}
}

func TestStationsHandler_SetsCORSHeaders(t *testing.T) {
	lat, long := 52.52, 13.405
	setupGlobalState(t,
		[]*Station{
			{ID: "STN001", Name: "Berlin", Latitude: &lat, Longitude: &long},
		},
		map[string]*StationInventory{
			"STN001": {FirstYear: 1900, LastYear: 2023},
		},
	)

	req := httptest.NewRequest(http.MethodGet, "/stations?lat=52.52&long=13.405&radius=100&limit=10&start=1950&end=2020", nil)
	rec := httptest.NewRecorder()

	stationsHandler(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("expected CORS origin header '*'")
	}
	if rec.Header().Get("Access-Control-Allow-Headers") != "Content-Type" {
		t.Error("expected CORS headers header 'Content-Type'")
	}
}

// ─── stationHandler Tests ──────────────────────────────────────────────────────

func TestStationHandler_MissingID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/station", nil)
	rec := httptest.NewRecorder()

	stationHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}

	var resp Response
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.ErrorMsg == "" {
		t.Error("expected error message for missing ID")
	}
}

func TestStationHandler_SetsCORSHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/station", nil)
	rec := httptest.NewRecorder()

	stationHandler(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("expected CORS origin header '*'")
	}
}

// ─── Mock HTTP Server Helper ───────────────────────────────────────────────────

// newMockS3Server creates a mock HTTP server that serves CSV data for station IDs.
// The handler maps station IDs to CSV content.
func newMockS3Server(csvByStation map[string]string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// URL pattern: /{stationID}.csv
		path := r.URL.Path
		// Extract station ID from path like /USW00094728.csv
		if len(path) > 5 && path[len(path)-4:] == ".csv" {
			id := path[1 : len(path)-4]
			if csv, ok := csvByStation[id]; ok {
				w.Header().Set("Content-Type", "text/csv")
				w.Write([]byte(csv))
				return
			}
		}
		http.NotFound(w, r)
	}))
}

// setupCache resets the global cache for testing. Cleans up after test completes.
func setupCache(t *testing.T) {
	oldCache := cache
	cache = &stationCache{entries: make(map[string]cacheEntry)}
	t.Cleanup(func() {
		cache = oldCache
	})
}

// setupBaseURL overrides the global baseURL for testing. Cleans up after test completes.
func setupBaseURL(t *testing.T, url string) {
	oldURL := baseURL
	baseURL = url
	t.Cleanup(func() {
		baseURL = oldURL
	})
}

// ─── loadStationData Tests (with mock HTTP server) ─────────────────────────────

func TestLoadStationData_ParsesCSVCorrectly(t *testing.T) {
	csvData := `"ID","DATE","ELEMENT","DATA_VALUE","M_FLAG","Q_FLAG","S_FLAG","OBS_TIME"
"USW00094728","20200101","TMIN",50,"","","S","0700"
"USW00094728","20200101","TMAX",120,"","","S","0700"
"USW00094728","20200102","TMIN",30,"","","S","0700"
"USW00094728","20200102","PRCP",5,"","","S","0700"
`
	server := newMockS3Server(map[string]string{"USW00094728": csvData})
	defer server.Close()

	result, err := loadStationData(server.URL, "USW00094728")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 3 entries (2 TMIN + 1 TMAX), PRCP filtered out
	if len(result) != 3 {
		t.Fatalf("expected 3 records (TMIN+TMAX only), got %d", len(result))
	}

	// Verify first record
	if result[0].ElementType != "TMIN" || result[0].Value != 50 {
		t.Errorf("first record: expected TMIN/50, got %s/%d", result[0].ElementType, result[0].Value)
	}
	if result[0].Date != time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC) {
		t.Errorf("first record: wrong date: %v", result[0].Date)
	}
}

func TestLoadStationData_FiltersTMINandTMAXOnly(t *testing.T) {
	csvData := `"ID","DATE","ELEMENT","DATA_VALUE","M_FLAG","Q_FLAG","S_FLAG","OBS_TIME"
"STN001","20200101","TMIN",50,"","","S",""
"STN001","20200101","TMAX",120,"","","S",""
"STN001","20200101","PRCP",5,"","","S",""
"STN001","20200101","SNOW",0,"","","S",""
"STN001","20200101","SNWD",0,"","","S",""
`
	server := newMockS3Server(map[string]string{"STN001": csvData})
	defer server.Close()

	result, err := loadStationData(server.URL, "STN001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 records (TMIN+TMAX), got %d", len(result))
	}
	for _, r := range result {
		if r.ElementType != "TMIN" && r.ElementType != "TMAX" {
			t.Errorf("unexpected element type: %s", r.ElementType)
		}
	}
}

func TestLoadStationData_Minus9999IsSkipped(t *testing.T) {
	csvData := `"ID","DATE","ELEMENT","DATA_VALUE","M_FLAG","Q_FLAG","S_FLAG","OBS_TIME"
"STN001","20200101","TMIN",100,"","","S",""
"STN001","20200102","TMIN",-9999,"","","S",""
"STN001","20200103","TMAX",250,"","","S",""
"STN001","20200104","TMAX",-9999,"","","S",""
`
	server := newMockS3Server(map[string]string{"STN001": csvData})
	defer server.Close()

	result, err := loadStationData(server.URL, "STN001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only 2 valid records (the two -9999 values should be filtered)
	if len(result) != 2 {
		t.Fatalf("expected 2 records (-9999 filtered), got %d", len(result))
	}
	if result[0].Value != 100 {
		t.Errorf("expected first value 100, got %d", result[0].Value)
	}
	if result[1].Value != 250 {
		t.Errorf("expected second value 250, got %d", result[1].Value)
	}
}

func TestLoadStationData_MalformedLines(t *testing.T) {
	csvData := `"ID","DATE","ELEMENT","DATA_VALUE","M_FLAG","Q_FLAG","S_FLAG","OBS_TIME"
"STN001","20200101","TMIN",100,"","","S",""
"STN001","baddate","TMIN",200,"","","S",""
"STN001","20200103","TMIN",notanumber,"","","S",""
"STN001","20200104","TMIN",300,"","","S",""
`
	server := newMockS3Server(map[string]string{"STN001": csvData})
	defer server.Close()

	result, err := loadStationData(server.URL, "STN001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// baddate and notanumber lines should be skipped gracefully
	if len(result) != 2 {
		t.Errorf("expected 2 valid records (malformed skipped), got %d", len(result))
	}
}

func TestLoadStationData_ShortLines(t *testing.T) {
	csvData := `"ID","DATE","ELEMENT","DATA_VALUE","M_FLAG","Q_FLAG","S_FLAG","OBS_TIME"
"STN001","20200101","TMIN",100,"","","S",""
"STN001","20200102"
"STN001","20200103","TMIN",200,"","","S",""
`
	server := newMockS3Server(map[string]string{"STN001": csvData})
	defer server.Close()

	result, err := loadStationData(server.URL, "STN001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Short line should be skipped (len < 4 columns)
	if len(result) != 2 {
		t.Errorf("expected 2 records (short line skipped), got %d", len(result))
	}
}

func TestLoadStationData_HTTP404(t *testing.T) {
	server := newMockS3Server(map[string]string{}) // no stations registered
	defer server.Close()

	_, err := loadStationData(server.URL, "NONEXISTENT")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

func TestLoadStationData_NetworkError(t *testing.T) {
	// Use an invalid URL that will fail to connect
	_, err := loadStationData("http://127.0.0.1:1", "STN001")
	if err == nil {
		t.Fatal("expected network error, got nil")
	}
}

func TestLoadStationData_EmptyCSV(t *testing.T) {
	csvData := `"ID","DATE","ELEMENT","DATA_VALUE","M_FLAG","Q_FLAG","S_FLAG","OBS_TIME"
`
	server := newMockS3Server(map[string]string{"STN001": csvData})
	defer server.Close()

	result, err := loadStationData(server.URL, "STN001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 records for empty CSV body, got %d", len(result))
	}
}

func TestLoadStationData_SkipsHeaderRow(t *testing.T) {
	// The header row contains "ELEMENT" which is not "TMIN"/"TMAX",
	// so it should be skipped by the reader.Read() call
	csvData := `"ID","DATE","ELEMENT","DATA_VALUE","M_FLAG","Q_FLAG","S_FLAG","OBS_TIME"
"STN001","20200101","TMIN",100,"","","S",""
`
	server := newMockS3Server(map[string]string{"STN001": csvData})
	defer server.Close()

	result, err := loadStationData(server.URL, "STN001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 record, got %d", len(result))
	}
}

func TestLoadStationData_MultipleYearsOfData(t *testing.T) {
	csvData := `"ID","DATE","ELEMENT","DATA_VALUE","M_FLAG","Q_FLAG","S_FLAG","OBS_TIME"
"STN001","20180615","TMIN",150,"","","S",""
"STN001","20180615","TMAX",300,"","","S",""
"STN001","20190715","TMIN",180,"","","S",""
"STN001","20190715","TMAX",320,"","","S",""
"STN001","20200815","TMIN",200,"","","S",""
"STN001","20200815","TMAX",350,"","","S",""
`
	server := newMockS3Server(map[string]string{"STN001": csvData})
	defer server.Close()

	result, err := loadStationData(server.URL, "STN001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 6 {
		t.Errorf("expected 6 records, got %d", len(result))
	}

	// Verify the data feeds correctly into annual calculation
	annual := calculateAnnualAvg(result)
	if len(annual) != 3 {
		t.Errorf("expected 3 years from loaded data, got %d", len(annual))
	}
}

// ─── Cache Tests ───────────────────────────────────────────────────────────────

func TestGetStationData_CacheMiss_FetchesAndCaches(t *testing.T) {
	setupCache(t)

	var fetchCount int32
	csvData := `"ID","DATE","ELEMENT","DATA_VALUE","M_FLAG","Q_FLAG","S_FLAG","OBS_TIME"
"STN001","20200101","TMIN",100,"","","S",""
"STN001","20200101","TMAX",250,"","","S",""
`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&fetchCount, 1)
		w.Header().Set("Content-Type", "text/csv")
		w.Write([]byte(csvData))
	}))
	defer server.Close()
	setupBaseURL(t, server.URL)

	// First call - cache miss, should fetch from server
	data, err := getStationData("STN001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) != 2 {
		t.Errorf("expected 2 records, got %d", len(data))
	}
	if atomic.LoadInt32(&fetchCount) != 1 {
		t.Errorf("expected 1 fetch on cache miss, got %d", fetchCount)
	}

	// Verify entry is now in cache
	cache.mu.RLock()
	_, exists := cache.entries["STN001"]
	cache.mu.RUnlock()
	if !exists {
		t.Error("expected cache entry after first fetch")
	}
}

func TestGetStationData_CacheHit_NoSecondFetch(t *testing.T) {
	setupCache(t)

	var fetchCount int32
	csvData := `"ID","DATE","ELEMENT","DATA_VALUE","M_FLAG","Q_FLAG","S_FLAG","OBS_TIME"
"STN001","20200101","TMIN",100,"","","S",""
`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&fetchCount, 1)
		w.Header().Set("Content-Type", "text/csv")
		w.Write([]byte(csvData))
	}))
	defer server.Close()
	setupBaseURL(t, server.URL)

	// First call - fetches from server
	_, err := getStationData("STN001")
	if err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}

	// Second call - should use cache
	data, err := getStationData("STN001")
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	if len(data) != 1 {
		t.Errorf("expected 1 record from cache, got %d", len(data))
	}
	if atomic.LoadInt32(&fetchCount) != 1 {
		t.Errorf("expected only 1 fetch (cache hit on second call), got %d", fetchCount)
	}
}

func TestGetStationData_CacheExpired_Refetches(t *testing.T) {
	setupCache(t)

	var fetchCount int32
	csvData := `"ID","DATE","ELEMENT","DATA_VALUE","M_FLAG","Q_FLAG","S_FLAG","OBS_TIME"
"STN001","20200101","TMIN",100,"","","S",""
`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&fetchCount, 1)
		w.Header().Set("Content-Type", "text/csv")
		w.Write([]byte(csvData))
	}))
	defer server.Close()
	setupBaseURL(t, server.URL)

	// First fetch
	_, err := getStationData("STN001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Manually expire the cache entry
	cache.mu.Lock()
	entry := cache.entries["STN001"]
	cache.entries["STN001"] = cacheEntry{data: entry.data, fetchedAt: time.Now().Add(-2 * cacheTTL)}
	cache.mu.Unlock()

	// Second fetch should re-fetch from server because cache is expired
	_, err = getStationData("STN001")
	if err != nil {
		t.Fatalf("unexpected error after expiry: %v", err)
	}
	if atomic.LoadInt32(&fetchCount) != 2 {
		t.Errorf("expected 2 fetches (expired cache), got %d", fetchCount)
	}
}

func TestGetStationData_FetchError_ReturnsError(t *testing.T) {
	setupCache(t)
	setupBaseURL(t, "http://127.0.0.1:1") // invalid, will fail to connect

	_, err := getStationData("STN001")
	if err == nil {
		t.Fatal("expected error for network failure, got nil")
	}
}

func TestCache_ConcurrentAccess(t *testing.T) {
	setupCache(t)

	csvData := `"ID","DATE","ELEMENT","DATA_VALUE","M_FLAG","Q_FLAG","S_FLAG","OBS_TIME"
"STN001","20200101","TMIN",100,"","","S",""
`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		w.Write([]byte(csvData))
	}))
	defer server.Close()
	setupBaseURL(t, server.URL)

	// Spawn multiple goroutines calling getStationData concurrently
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("STN%03d", i%5) // 5 different station IDs
			_, _ = getStationData(id)
		}(i)
	}
	wg.Wait()
	// If we get here without a panic/deadlock, concurrency is handled correctly
}

// ─── stationHandler Integration Tests (with mock server) ──────────────────────

func TestStationHandler_ValidID_ReturnsAnnualAndSeasonal(t *testing.T) {
	setupCache(t)

	// Pre-populate cache with known data so stationHandler uses it via getStationData
	rawData := []RawStationData{
		{Date: time.Date(2020, 1, 15, 0, 0, 0, 0, time.UTC), ElementType: "TMIN", Value: -50},
		{Date: time.Date(2020, 1, 15, 0, 0, 0, 0, time.UTC), ElementType: "TMAX", Value: 30},
		{Date: time.Date(2020, 7, 15, 0, 0, 0, 0, time.UTC), ElementType: "TMIN", Value: 180},
		{Date: time.Date(2020, 7, 15, 0, 0, 0, 0, time.UTC), ElementType: "TMAX", Value: 300},
	}
	cache.mu.Lock()
	cache.entries["TESTSTATION"] = cacheEntry{data: rawData, fetchedAt: time.Now()}
	cache.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/station?id=TESTSTATION", nil)
	rec := httptest.NewRecorder()

	stationHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp Response
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.ErrorMsg != "" {
		t.Errorf("expected no error, got %q", resp.ErrorMsg)
	}

	// Verify the data field contains annual and seasonal keys
	dataMap, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected data to be a map, got %T", resp.Data)
	}
	if _, ok := dataMap["annual"]; !ok {
		t.Error("expected 'annual' key in response data")
	}
	if _, ok := dataMap["seasonal"]; !ok {
		t.Error("expected 'seasonal' key in response data")
	}
}

func TestStationHandler_FetchesViaCache(t *testing.T) {
	setupCache(t)

	var fetchCount int32
	csvData := `"ID","DATE","ELEMENT","DATA_VALUE","M_FLAG","Q_FLAG","S_FLAG","OBS_TIME"
"LIVE001","20200601","TMIN",180,"","","S",""
"LIVE001","20200601","TMAX",300,"","","S",""
`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&fetchCount, 1)
		w.Header().Set("Content-Type", "text/csv")
		w.Write([]byte(csvData))
	}))
	defer server.Close()
	setupBaseURL(t, server.URL)

	// First request - should fetch from mock server
	req := httptest.NewRequest(http.MethodGet, "/station?id=LIVE001", nil)
	rec := httptest.NewRecorder()
	stationHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if atomic.LoadInt32(&fetchCount) != 1 {
		t.Errorf("expected 1 fetch, got %d", fetchCount)
	}

	// Second request - should use cache
	req2 := httptest.NewRequest(http.MethodGet, "/station?id=LIVE001", nil)
	rec2 := httptest.NewRecorder()
	stationHandler(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec2.Code)
	}
	if atomic.LoadInt32(&fetchCount) != 1 {
		t.Errorf("expected still 1 fetch (cache hit), got %d", fetchCount)
	}
}

func TestStationHandler_FetchError_Returns500(t *testing.T) {
	setupCache(t)
	setupBaseURL(t, "http://127.0.0.1:1") // will fail

	req := httptest.NewRequest(http.MethodGet, "/station?id=FAILSTN", nil)
	rec := httptest.NewRecorder()
	stationHandler(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}

	var resp Response
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.ErrorMsg == "" {
		t.Error("expected error message for fetch failure")
	}
}

// ─── JSON Serialization Tests ──────────────────────────────────────────────────

func TestResponse_JSONSerialization(t *testing.T) {
	resp := Response{
		Data:     []*Station{{ID: "STN001", Name: "Berlin"}},
		ErrorMsg: "",
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal Response: %v", err)
	}

	var decoded Response
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal Response: %v", err)
	}
	if decoded.ErrorMsg != "" {
		t.Errorf("expected empty error, got %q", decoded.ErrorMsg)
	}
}

func TestAnnualStationData_JSONOmitsNilPointers(t *testing.T) {
	data := AnnualStationData{Year: 2020, TMin: nil, TMax: nil}
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	// TMin and TMax are *float64 without omitempty, so they should appear as null
	var decoded map[string]interface{}
	json.Unmarshal(b, &decoded)
	if _, ok := decoded["tmin"]; !ok {
		t.Error("expected tmin key in JSON")
	}
	_ = s
}

func TestStationDetailResponse_JSONStructure(t *testing.T) {
	tmin := 10.0
	tmax := 25.0
	detail := StationDetailResponse{
		Annual: []*AnnualStationData{
			{Year: 2020, TMin: &tmin, TMax: &tmax},
		},
		Seasonal: []*SeasonalStationData{
			{Year: 2020, Season: "Summer", TMin: &tmin, TMax: &tmax},
		},
	}
	b, err := json.Marshal(detail)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]interface{}
	json.Unmarshal(b, &decoded)
	if _, ok := decoded["annual"]; !ok {
		t.Error("expected 'annual' key in JSON")
	}
	if _, ok := decoded["seasonal"]; !ok {
		t.Error("expected 'seasonal' key in JSON")
	}
}

func TestStationDetailResponse_OmitsEmptyArrays(t *testing.T) {
	detail := StationDetailResponse{}
	b, err := json.Marshal(detail)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]interface{}
	json.Unmarshal(b, &decoded)
	// omitempty on slices means nil slices are omitted
	if _, ok := decoded["annual"]; ok {
		t.Error("expected 'annual' to be omitted when nil")
	}
	if _, ok := decoded["seasonal"]; ok {
		t.Error("expected 'seasonal' to be omitted when nil")
	}
}

// ─── Edge Case Tests ───────────────────────────────────────────────────────────

func TestCalculateAnnualAvg_LargeDataset(t *testing.T) {
	// Generate 365 days * 20 years of data
	var raw []RawStationData
	for year := 2000; year < 2020; year++ {
		for day := 1; day <= 365; day++ {
			date := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, day-1)
			raw = append(raw, RawStationData{Date: date, ElementType: "TMIN", Value: 100})
			raw = append(raw, RawStationData{Date: date, ElementType: "TMAX", Value: 300})
		}
	}

	result := calculateAnnualAvg(raw)
	if len(result) != 20 {
		t.Errorf("expected 20 years, got %d", len(result))
	}
	// Each year has constant values, so avg should be exact
	for _, r := range result {
		if !approxEqual(*r.TMin, 10.0, 0.01) {
			t.Errorf("year %d: expected TMin ~10.0, got %f", r.Year, *r.TMin)
		}
		if !approxEqual(*r.TMax, 30.0, 0.01) {
			t.Errorf("year %d: expected TMax ~30.0, got %f", r.Year, *r.TMax)
		}
	}
}

func TestFindStations_ZeroRadius(t *testing.T) {
	lat, long := 52.52, 13.405
	setupGlobalState(t,
		[]*Station{
			{ID: "STN001", Name: "Same Spot", Latitude: &lat, Longitude: &long},
		},
		map[string]*StationInventory{
			"STN001": {FirstYear: 1900, LastYear: 2023},
		},
	)

	// Radius 0 - the station at the exact coordinates should match (distance ~0)
	result, err := findStations(52.52, 13.405, 0, 10, 1950, 2020)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Distance will be ~0.0 which is <= 0, so it should be included
	if len(result) != 1 {
		t.Errorf("expected 1 station at zero radius (same coords), got %d", len(result))
	}
}

func TestFindStations_InventoryEndYearFilter(t *testing.T) {
	lat, long := 52.52, 13.405
	setupGlobalState(t,
		[]*Station{
			{ID: "STN001", Name: "Old Station", Latitude: &lat, Longitude: &long},
		},
		map[string]*StationInventory{
			"STN001": {FirstYear: 1900, LastYear: 1950},
		},
	)

	// Request endYear=2020 but station data ends at 1950
	result, _ := findStations(52.52, 13.405, 100, 10, 1900, 2020)
	if len(result) != 0 {
		t.Errorf("expected 0 stations (inventory ends before endYear), got %d", len(result))
	}

	// Request endYear=1950 - should now match
	result, _ = findStations(52.52, 13.405, 100, 10, 1900, 1950)
	if len(result) != 1 {
		t.Errorf("expected 1 station, got %d", len(result))
	}
}

func TestCalculateSeasonalAvg_DecemberIsWinter(t *testing.T) {
	raw := []RawStationData{
		{Date: time.Date(2020, 12, 15, 0, 0, 0, 0, time.UTC), ElementType: "TMIN", Value: -50},
	}
	result := calculateSeasonalAvg(raw)
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if result[0].Season != "Winter" {
		t.Errorf("expected December to be Winter, got %s", result[0].Season)
	}
	// December 2020 should be attributed to year 2020 (current code behavior)
	if result[0].Year != 2020 {
		t.Errorf("expected year 2020, got %d", result[0].Year)
	}
}

func TestFindStations_EqualDistanceSorting(t *testing.T) {
	// Two stations at the exact same coordinates -> distance == 0 for both
	lat, long := 52.52, 13.405
	setupGlobalState(t,
		[]*Station{
			{ID: "STN_B", Name: "Station B", Latitude: &lat, Longitude: &long},
			{ID: "STN_A", Name: "Station A", Latitude: &lat, Longitude: &long},
		},
		map[string]*StationInventory{
			"STN_B": {FirstYear: 1900, LastYear: 2023},
			"STN_A": {FirstYear: 1900, LastYear: 2023},
		},
	)

	result, err := findStations(52.52, 13.405, 100, 10, 1950, 2020)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 stations, got %d", len(result))
	}
	// Both distances should be 0
	if result[0].Distance != 0 || result[1].Distance != 0 {
		t.Errorf("expected both distances to be 0, got %.4f and %.4f",
			result[0].Distance, result[1].Distance)
	}
}

// ─── countStationsInRadius Tests ───────────────────────────────────────────────

func TestCountStationsInRadius_NoStationsNearby(t *testing.T) {
	lat, long := 0.0, 0.0
	setupGlobalState(t,
		[]*Station{
			{ID: "STN001", Name: "Far Away", Latitude: &lat, Longitude: &long},
		},
		map[string]*StationInventory{},
	)

	count := countStationsInRadius(52.52, 13.405, 10)
	if count != 0 {
		t.Errorf("expected 0 stations in radius, got %d", count)
	}
}

func TestCountStationsInRadius_StationsNearby(t *testing.T) {
	lat1, long1 := 52.52, 13.405
	lat2, long2 := 52.53, 13.41
	lat3, long3 := 0.0, 0.0
	setupGlobalState(t,
		[]*Station{
			{ID: "STN001", Name: "Berlin 1", Latitude: &lat1, Longitude: &long1},
			{ID: "STN002", Name: "Berlin 2", Latitude: &lat2, Longitude: &long2},
			{ID: "STN003", Name: "Far Away", Latitude: &lat3, Longitude: &long3},
		},
		map[string]*StationInventory{},
	)

	count := countStationsInRadius(52.52, 13.405, 50)
	if count != 2 {
		t.Errorf("expected 2 stations in radius, got %d", count)
	}
}

func TestCountStationsInRadius_SkipsNilLatLong(t *testing.T) {
	lat := 52.52
	setupGlobalState(t,
		[]*Station{
			{ID: "STN001", Name: "No coords", Latitude: nil, Longitude: nil},
			{ID: "STN002", Name: "Partial", Latitude: &lat, Longitude: nil},
		},
		map[string]*StationInventory{},
	)

	count := countStationsInRadius(52.52, 13.405, 100)
	if count != 0 {
		t.Errorf("expected 0 stations (nil coords), got %d", count)
	}
}

// ─── stationsHandler Error Message Tests ───────────────────────────────────────

func TestStationsHandler_NoStationsInArea_ReturnsAreaMessage(t *testing.T) {
	setupGlobalState(t, []*Station{}, map[string]*StationInventory{})

	req := httptest.NewRequest(http.MethodGet, "/stations?lat=52.52&long=13.405&radius=10&limit=10&start=1950&end=2020", nil)
	rec := httptest.NewRecorder()

	stationsHandler(rec, req)

	var resp Response
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.ErrorMsg == "" {
		t.Error("expected error message when no stations found")
	}
	expected := "No stations found in this area. Try increasing the radius."
	if resp.ErrorMsg != expected {
		t.Errorf("expected %q, got %q", expected, resp.ErrorMsg)
	}
}

func TestStationsHandler_StationsExistButNoDataInRange_ReturnsYearMessage(t *testing.T) {
	lat, long := 52.52, 13.405
	setupGlobalState(t,
		[]*Station{
			{ID: "STN001", Name: "Berlin", Latitude: &lat, Longitude: &long},
		},
		map[string]*StationInventory{
			// Station data only covers 1900-1950, not the requested 2000-2020
			"STN001": {FirstYear: 1900, LastYear: 1950},
		},
	)

	req := httptest.NewRequest(http.MethodGet, "/stations?lat=52.52&long=13.405&radius=100&limit=10&start=2000&end=2020", nil)
	rec := httptest.NewRecorder()

	stationsHandler(rec, req)

	var resp Response
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.ErrorMsg == "" {
		t.Error("expected error message about year range")
	}
	// Should mention stations exist but no data in range
	if resp.ErrorMsg == "No stations found in this area. Try increasing the radius." {
		t.Error("should NOT show area message when stations exist geographically")
	}
	// Check it contains relevant info
	if !contains(resp.ErrorMsg, "2000") || !contains(resp.ErrorMsg, "2020") {
		t.Errorf("expected message to mention year range, got %q", resp.ErrorMsg)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
