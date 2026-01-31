package main

//loading libaries
import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

type Response struct {
	Data     any    `json:"data"`
	ErrorMsg string `json:"errorMessage"`
}

// internal memory for each line
type RawStationData struct {
	Date        time.Time
	ElementType string
	Value       int
}

type AnnualStationData struct {
	Year int      `json:"year"`
	TMin *float64 `json:"tmin"`
	TMax *float64 `json:"tmax"`
}

type SeasonalStationData struct {
	Year   int      `json:"year"`
	Season string   `json:"season"`
	TMin   *float64 `json:"tmin"`
	TMax   *float64 `json:"tmax"`
}

type StationDetailResponse struct {
	Annual   []*AnnualStationData   `json:"annual,omitempty"`
	Seasonal []*SeasonalStationData `json:"seasonal,omitempty"`
}

type Station struct {
	ID        string   `json:"id,omitempty"`
	Name      string   `json:"name,omitempty"`
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
	Distance  float64  `json:"distance,omitempty"`
}

type StationInventory struct {
	FirstYear int
	LastYear  int
}

var inventoryMap = make(map[string]*StationInventory)
var allStations []*Station

// loading the inventory file on start up
func loadInventory() error {
	url := "https://noaa-ghcn-pds.s3.amazonaws.com/ghcnd-inventory.txt"
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("Netzwerkfehler: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Datei %s nicht gefunden (Status %d)", url, resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 45 {
			continue
		}

		element := strings.TrimSpace(line[31:35])
		if element != "TMAX" && element != "TMIN" {
			continue
		}

		id := strings.TrimSpace(line[0:11])
		firstYear, _ := strconv.Atoi(strings.TrimSpace(line[36:40]))
		lastYear, _ := strconv.Atoi(strings.TrimSpace(line[41:45]))

		if inv, exists := inventoryMap[id]; exists {
			if firstYear < inv.FirstYear {
				inv.FirstYear = firstYear
			}
			if lastYear > inv.LastYear {
				inv.LastYear = lastYear
			}
		} else {
			inventoryMap[id] = &StationInventory{FirstYear: firstYear, LastYear: lastYear}
		}
	}
	return nil
}

// loading the stations file on start up
func initStations() error {
	url := "https://noaa-ghcn-pds.s3.amazonaws.com/ghcnd-stations.txt"

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("Netzwerkfehler: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Datei %s nicht gefunden (Status %d)", url, resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 71 {
			continue
		}

		//parsing the information from ghcnd-stations.txt
		id := strings.TrimSpace(line[0:11])
		latStr := strings.TrimSpace(line[12:20])
		longStr := strings.TrimSpace(line[21:30])
		name := strings.TrimSpace(line[38:71])

		lat, _ := strconv.ParseFloat(latStr, 64)
		long, _ := strconv.ParseFloat(longStr, 64)

		//storing basisdata, distance remains unset -> will be calculated later
		s := &Station{
			ID:        id,
			Name:      name,
			Latitude:  &lat,
			Longitude: &long,
		}
		allStations = append(allStations, s)
	}
	return nil
}

// searching for specific stations on given input variables
func findStations(latUsr float64, longUsr float64, radius int, limit int, startYear int, endYear int) ([]*Station, error) {
	var stations []*Station

	const earthRadius = 6371.0
	const p = math.Pi / 180

	for _, s := range allStations {
		if s.Latitude == nil || s.Longitude == nil {
			continue
		}

		lat := *s.Latitude
		long := *s.Longitude

		//calculating distance with haversine formula
		dLat := (latUsr - lat) * p
		dLong := (longUsr - long) * p

		a := math.Sin(dLat/2)*math.Sin(dLat/2) +
			math.Cos(latUsr*p)*math.Cos(lat*p)*
				math.Sin(dLong/2)*math.Sin(dLong/2)

		distance := earthRadius * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

		//filtering stations out of radius
		if distance > float64(radius) {
			continue
		}

		//filtering with inventory file if station has data available in given years
		inv, exists := inventoryMap[s.ID]
		if !exists || inv.FirstYear > startYear || inv.LastYear < endYear {
			continue
		}

		//adding station to list
		matchedStation := &Station{
			ID:        s.ID,
			Name:      s.Name,
			Latitude:  s.Latitude,
			Longitude: s.Longitude,
			Distance:  distance,
		}
		stations = append(stations, matchedStation)
	}

	//sorting the stations list
	slices.SortFunc(stations, func(a, b *Station) int {
		if a.Distance < b.Distance {
			return -1
		}
		if a.Distance > b.Distance {
			return 1
		}
		return 0
	})

	limStations := []*Station{}
	for i, x := range stations {
		limStations = append(limStations, x)
		if i+1 == limit {
			break
		}
	}
	return limStations, nil
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "OK")
}

// read user input
// filter station list
// write station (json)
func stationsHandler(w http.ResponseWriter, r *http.Request) {
	//cors handling
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	q := r.URL.Query()
	latStr := q.Get("lat")
	longStr := q.Get("long")
	radiusStr := q.Get("radius")
	limitStr := q.Get("limit")
	startDateStr := q.Get("start")
	endDateStr := q.Get("end")
	enc := json.NewEncoder(w)

	if latStr == "" {
		w.WriteHeader(http.StatusBadRequest)
		response := Response{Data: []*Station{}, ErrorMsg: "Bitte geben Sie den Breitengrad an."}
		enc.Encode(response)
		return
	}
	if longStr == "" {
		w.WriteHeader(http.StatusBadRequest)
		response := Response{Data: []*Station{}, ErrorMsg: "Bitte geben Sie den Längengrad an."}
		enc.Encode(response)
		return
	}
	if radiusStr == "" {
		w.WriteHeader(http.StatusBadRequest)
		response := Response{Data: []*Station{}, ErrorMsg: "Bitte geben Sie den Radius an."}
		enc.Encode(response)
		return
	}
	if limitStr == "" {
		w.WriteHeader(http.StatusBadRequest)
		response := Response{Data: []*Station{}, ErrorMsg: "Bitte geben Sie das Limit an."}
		enc.Encode(response)
		return
	}
	if startDateStr == "" {
		w.WriteHeader(http.StatusBadRequest)
		response := Response{Data: []*Station{}, ErrorMsg: "Bitte geben Sie das Anfangsjahr an."}
		enc.Encode(response)
		return
	}
	if endDateStr == "" {
		w.WriteHeader(http.StatusBadRequest)
		response := Response{Data: []*Station{}, ErrorMsg: "Bitte geben Sie das Endjahr an."}
		enc.Encode(response)
		return
	}
	lat, err := strconv.ParseFloat(latStr, 32)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		response := Response{Data: []*Station{}, ErrorMsg: "Bitte geben Sie eine gültige Zahl an"}
		enc.Encode(response)
		return
	}
	long, err := strconv.ParseFloat(longStr, 32)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		response := Response{Data: []*Station{}, ErrorMsg: "Bitte geben Sie eine gültige Zahl an"}
		enc.Encode(response)
		return
	}
	radius, err := strconv.Atoi(radiusStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		response := Response{Data: []*Station{}, ErrorMsg: "Bitte geben Sie eine gültige Zahl an"}
		enc.Encode(response)
		return
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		response := Response{Data: []*Station{}, ErrorMsg: "Bitte geben Sie eine gültige Zahl an"}
		enc.Encode(response)
		return
	}
	start, err := strconv.Atoi(startDateStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		response := Response{Data: []*Station{}, ErrorMsg: "Bitte geben Sie eine gültige Zahl an"}
		enc.Encode(response)
		return
	}
	end, err := strconv.Atoi(endDateStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		response := Response{Data: []*Station{}, ErrorMsg: "Bitte geben Sie eine gültige Zahl an"}
		enc.Encode(response)
		return
	}

	stationList, _ := findStations(lat, long, radius, limit, start, end)
	response := Response{Data: stationList, ErrorMsg: ""}
	enc.Encode(response)
}

func loadStationData(id string) ([]RawStationData, error) {
	url := fmt.Sprintf("https://noaa-ghcn-pds.s3.amazonaws.com/csv/by_station/%s.csv", id)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("Netzwerkfehler: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Station %s nicht gefunden (Status %d)", id, resp.StatusCode)
	}

	reader := csv.NewReader(resp.Body)
	var dataList []RawStationData
	const layout = "20060102"

	_, _ = reader.Read()

	for {
		line, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(line) < 4 {
			continue
		}

		//filtering for TMIN and TMAX only
		element := line[2]
		if element != "TMIN" && element != "TMAX" {
			continue
		}

		date, err := time.Parse(layout, line[1])
		if err != nil {
			continue
		}

		//skipping empty data
		val, err := strconv.Atoi(line[3])
		if err != nil || val == -9999 {
			continue
		}

		dataList = append(dataList, RawStationData{
			Date:        date,
			ElementType: element,
			Value:       val,
		})
	}
	return dataList, nil
}

// calculating yearly average for tmin and tmax
func calculateAnnualAvg(rawData []RawStationData) []*AnnualStationData {
	type Aggr struct {
		sumMin, countMin int
		sumMax, countMax int
	}
	stats := make(map[int]*Aggr)

	for _, d := range rawData {
		year := d.Date.Year()
		if _, ok := stats[year]; !ok {
			stats[year] = &Aggr{}
		}
		switch d.ElementType {
		case "TMIN":
			stats[year].sumMin += d.Value
			stats[year].countMin++
		case "TMAX":
			stats[year].sumMax += d.Value
			stats[year].countMax++
		}
	}
	var result []*AnnualStationData
	for year, val := range stats {
		sData := &AnnualStationData{Year: year}
		if val.countMin > 0 {
			avg := (float64(val.sumMin) / float64(val.countMin)) / 10
			avg = math.Round(avg*100) / 100
			sData.TMin = &avg
		}
		if val.countMax > 0 {
			avg := (float64(val.sumMax) / float64(val.countMax)) / 10
			avg = math.Round(avg*100) / 100
			sData.TMax = &avg
		}
		result = append(result, sData)
	}
	slices.SortFunc(result, func(a, b *AnnualStationData) int { return a.Year - b.Year })
	return result
}

// defining seasons and calculating seasonal average
func calculateSeasonalAvg(rawData []RawStationData) []*SeasonalStationData {
	type Aggr struct {
		sumMin, countMin int
		sumMax, countMax int
	}
	stats := make(map[string]*Aggr)

	for _, d := range rawData {
		month := d.Date.Month()
		year := d.Date.Year()
		var season string

		switch month {
		case time.March, time.April, time.May:
			season = "Spring"
		case time.June, time.July, time.August:
			season = "Summer"
		case time.September, time.October, time.November:
			season = "Autumn"
		case time.December:
			season = "Winter"
			year = year + 1
		case time.January, time.February:
			season = "Winter"
		default:
			continue
		}

		key := fmt.Sprintf("%d-%s", year, season)
		if _, ok := stats[key]; !ok {
			stats[key] = &Aggr{}
		}
		switch d.ElementType {
		case "TMIN":
			stats[key].sumMin += d.Value
			stats[key].countMin++
		case "TMAX":
			stats[key].sumMax += d.Value
			stats[key].countMax++
		}
	}

	var result []*SeasonalStationData
	for key, val := range stats {
		parts := strings.Split(key, "-")
		year, _ := strconv.Atoi(parts[0])
		season := parts[1]
		sData := &SeasonalStationData{Year: year, Season: season}

		if val.countMin > 0 {
			avg := (float64(val.sumMin) / float64(val.countMin)) / 10.0
			avg = math.Round(avg*100) / 100
			sData.TMin = &avg
		}
		if val.countMax > 0 {
			avg := (float64(val.sumMax) / float64(val.countMax)) / 10.0
			avg = math.Round(avg*100) / 100
			sData.TMax = &avg
		}
		result = append(result, sData)
	}
	slices.SortFunc(result, func(a, b *SeasonalStationData) int {
		if a.Year != b.Year {
			return a.Year - b.Year
		}
		order := map[string]int{"Winter": 1, "Spring": 2, "Summer": 3, "Autumn": 4}
		return order[a.Season] - order[b.Season]
	})
	return result
}

func stationHandler(w http.ResponseWriter, r *http.Request) {
	//cors handling
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	q := r.URL.Query()
	id := q.Get("id")
	enc := json.NewEncoder(w)

	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		response := Response{Data: nil, ErrorMsg: "Geben Sie die richtige ID an"}
		enc.Encode(response)
		return
	}

	rawData, err := loadStationData(id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		response := Response{Data: nil, ErrorMsg: err.Error()}
		enc.Encode(response)
		return
	}

	annualData := calculateAnnualAvg(rawData)
	seasonalData := calculateSeasonalAvg(rawData)

	detailData := StationDetailResponse{
		Annual:   annualData,
		Seasonal: seasonalData,
	}

	response := Response{Data: detailData, ErrorMsg: ""}
	enc.Encode(response)
}

func main() {
	err := loadInventory()
	if err != nil {
		// file for rough filtering
		fmt.Printf("Fehler beim Laden des Inventars: %v\n", err)
		return
	}
	err = initStations()
	if err != nil {
		fmt.Printf("Fehler beim Laden der Stationen: %v\n", err)
		return
	}
	http.HandleFunc("/status", statusHandler)
	fmt.Println("Starting server on :8080")
	http.HandleFunc("/stations", stationsHandler)
	http.HandleFunc("/station", stationHandler)
	http.ListenAndServe(":8080", nil)
}
