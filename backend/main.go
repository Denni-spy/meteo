package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type Station struct {
	ID        string  `json:"id,omitempty"`
	Name      string  `json:"name,omitempty"`
	Latitude  float32 `json:"latitude,omitempty"`
	Longitude float32 `json:"longitude,omitempty"`
	Distance  float32 `json:"distance,omitempty"`
}

func loadStations() ([]Station, error) {
	var stations []Station

	file, err := os.Open("data/ghcnd-stations.txt")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// Eine Zeile im GHCN-Format muss mindestens 71 Zeichen lang sein
		if len(line) < 71 {
			continue
		}

		// Zerschneiden der Zeile an festen Positionen (Fixed Width)
		id := strings.TrimSpace(line[0:11])
		latStr := strings.TrimSpace(line[12:20])
		longStr := strings.TrimSpace(line[21:30])
		name := strings.TrimSpace(line[38:71])

		// Text in Zahlen umwandeln
		lat, _ := strconv.ParseFloat(latStr, 32)
		long, _ := strconv.ParseFloat(longStr, 32)

		// Station zur Liste hinzufügen
		s := Station{
			ID:        id,
			Name:      name,
			Latitude:  float32(lat),
			Longitude: float32(long),
		}
		stations = append(stations, s)
	}
	return stations, nil
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Hello, World!")
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "OK")
}

func stationHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	lat := q.Get("lat")
	long := q.Get("long")
	radius := q.Get("radius")

	if lat == "" || long == "" || radius == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, "Fehler: Bitte geben Sie lat, long und radius an.")
		return
	}

	fmt.Println("Suche gestartet: Lat=", lat, "Long=", long, "Radius=", radius)

	data := Station{
		ID:        "GME00102380",
		Name:      "NURNBERG",
		Latitude:  49.47,
		Longitude: 10.99,
		Distance:  5.2, // km
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func main() {
	http.HandleFunc("/", helloHandler)        //Root URL
	http.HandleFunc("/status", statusHandler) //Status URL
	fmt.Println("Starting server on :8080")
	fmt.Println("Lade Stationsdaten")
	allStations, err := loadStations()
	if err != nil {
		fmt.Println("Fehler beim Laden der Datei:", err)
		return
	}
	fmt.Printf("Erfolg! %d Stationen geladen.\n", len(allStations))
	http.HandleFunc("/station", stationHandler)
	http.ListenAndServe(":8080", nil)
}
