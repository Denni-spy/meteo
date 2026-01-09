package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

type Response struct {
	Data     any    `json:"data"`
	ErrorMsg string `json:"errorMessage"`
}

type StationData struct {
	Date      time.Time `json:"date"`
	DataValue int       `json:"dataValue"`
}

type MonthlyStationData struct {
	Year     string  `json:"year"`
	Month    string  `json:"month"`
	AvgValue float64 `json:"value"`
}

type Station struct {
	ID        string   `json:"id,omitempty"`
	Name      string   `json:"name,omitempty"`
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
	Distance  float64  `json:"distance,omitempty"`
}

func loadStations(latUsr float64, longUsr float64, radius int, limit int) ([]*Station, error) {
	var stations []*Station

	file, err := os.Open("data/ghcnd-stations.txt")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 71 {
			continue
		}

		// slicing
		id := strings.TrimSpace(line[0:11])
		latStr := strings.TrimSpace(line[12:20])
		longStr := strings.TrimSpace(line[21:30])
		name := strings.TrimSpace(line[38:71])

		// parse string to float
		lat, _ := strconv.ParseFloat(latStr, 64)
		long, _ := strconv.ParseFloat(longStr, 64)

		const earthRadius = 6371.0
		const p = math.Pi / 180

		// calculate deltas
		dLat := (lat - latUsr) * p
		dLong := (long - longUsr) * p

		// Haversine formula
		a := math.Sin(dLat/2)*math.Sin(dLat/2) +
			math.Cos(latUsr*p)*math.Cos(lat*p)*
				math.Sin(dLong/2)*math.Sin(dLong/2)

		// calculate distance
		distance := earthRadius * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

		// append station to list
		if distance <= float64(radius) {
			s := &Station{
				ID:        id,
				Name:      name,
				Latitude:  &lat,
				Longitude: &long,
				Distance:  distance,
			}
			stations = append(stations, s)
		}
	}

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
	_, err = strconv.Atoi(startDateStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		response := Response{Data: []*Station{}, ErrorMsg: "Bitte geben Sie eine gültige Zahl an"}
		enc.Encode(response)
		return
	}
	_, err = strconv.Atoi(endDateStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		response := Response{Data: []*Station{}, ErrorMsg: "Bitte geben Sie eine gültige Zahl an"}
		enc.Encode(response)
		return
	}

	stationList, _ := loadStations(lat, long, radius, limit)
	response := Response{Data: stationList, ErrorMsg: ""}
	enc.Encode(response)
}

func loadStationData(_ string) ([]*StationData, error) {
	file, err := os.Open("data/GME00102380.csv")
	if err != nil {
		return []*StationData{}, err
	}
	defer file.Close()

	reader := csv.NewReader(file)

	var dataList []*StationData
	_, _ = reader.Read()

	const layout = "20060102"

	for {
		line, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return []*StationData{}, err
		}
		dateStr := line[1]
		valueStr := line[3]
		date, err := time.Parse(layout, dateStr)
		if err != nil {
			return []*StationData{}, err
		}
		// string to int
		value, err := strconv.Atoi(valueStr)
		if err != nil {
			return []*StationData{}, err
		}

		// add data to list
		data := &StationData{
			Date:      date,
			DataValue: value,
		}
		dataList = append(dataList, data)
	}
	return dataList, nil
}

func calculateAvgTemp(tempData []*StationData) []*MonthlyStationData {
	type stats struct {
		sum   int
		count int
	}
	// "1850-12" -> stats
	monthlyStats := make(map[string]*stats)
	for _, x := range tempData {
		key := x.Date.Format("2006-01")
		_, ok := monthlyStats[key]
		if !ok {
			monthlyStats[key] = &stats{}
		}
		monthlyStats[key].count++
		monthlyStats[key].sum += x.DataValue
	}
	avgTemp := []*MonthlyStationData{}
	for k, v := range monthlyStats {
		avg := (float64(v.sum) / float64(v.count)) / 10.0
		parts := strings.Split(k, "-")
		avgTemp = append(avgTemp, &MonthlyStationData{AvgValue: avg, Year: parts[0], Month: parts[1]})
	}
	slices.SortFunc(avgTemp, func(a, b *MonthlyStationData) int {
		layout := "200601"
		aDate, err := time.Parse(layout, a.Year+a.Month)
		if err != nil {
			return -1
		}
		bDate, err := time.Parse(layout, b.Year+b.Month)
		if err != nil {
			return 1
		}
		return aDate.Compare(bDate)
	})
	return avgTemp
}

func stationHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	id := q.Get("id")
	enc := json.NewEncoder(w)

	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		response := Response{Data: nil, ErrorMsg: "Geben Sie die richtige ID an"}
		enc.Encode(response)
		return
	}
	data, err := loadStationData(id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		response := Response{Data: nil, ErrorMsg: err.Error()}
		enc.Encode(response)
		return
	}
	avgData := calculateAvgTemp(data)
	response := Response{Data: avgData, ErrorMsg: ""}
	enc.Encode(response)
}

func main() {
	http.HandleFunc("/status", statusHandler)
	fmt.Println("Starting server on :8080")
	fmt.Println("Lade Stationsdaten")
	http.HandleFunc("/stations", stationsHandler)
	http.HandleFunc("/station", stationHandler)
	http.ListenAndServe(":8080", nil)
}
