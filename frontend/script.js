// initialize map
var map = L.map('map').setView([51.1657, 10.4515], 5);
var redIcon = new L.Icon({
    iconUrl: 'https://raw.githubusercontent.com/pointhi/leaflet-color-markers/master/img/marker-icon-2x-red.png',
    shadowUrl: 'https://cdnjs.cloudflare.com/ajax/libs/leaflet/0.7.7/images/marker-shadow.png',
    iconSize: [25, 41],
    iconAnchor: [12, 41],
    popupAnchor: [1, -34],
    shadowSize: [41, 41]
});
L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png').addTo(map);

let aktuellerKreis = null;
let aktuellerMarker = null;
let stationMarkers = [];

const form = document.getElementById('meteoForm');

form.addEventListener('submit', function (event) {
    event.preventDefault();

    //reading user input
    const lat = parseFloat(document.getElementById('latitude').value);
    const long = parseFloat(document.getElementById('longitude').value); 
    const radKm = parseInt(document.getElementById('radius').value);
    const limit = parseInt(document.getElementById('selection').value);
    const start = parseInt(document.getElementById('start').value);      
    const end = parseInt(document.getElementById('end').value);          
  

    //validating user input
    if (isNaN(lat) || isNaN(long) || isNaN(radKm) || isNaN(start) || isNaN(end) || isNaN(limit)) {
        alert("Please fill in all fields (latitude, longitude, radius, limit, start, end).");
        return;
    }

    if (radKm <= 0 || radKm > 100) {
        alert("The radius must be between 1 and 100 km.");
        return;
    }

    if (start < 1750 || start > 2025) {
        alert("Please enter a start year between 1750 and 2025.");
        return;
    }

    if (end < 1750 || end > 2025) {
        alert("Please enter an end year between 1750 and 2025.");
        return;
    }

    if (start > end) {
        alert("The start year must be before the end year.");
        return;
    }

    //clean up map
    if (aktuellerKreis) {
        map.removeLayer(aktuellerKreis);
    }
    if (aktuellerMarker) {
        map.removeLayer(aktuellerMarker);
    }
    //remove stations marker
    stationMarkers.forEach(marker => map.removeLayer(marker));
    stationMarkers = [];

    //creating new visuals
    aktuellerKreis = L.circle([lat, long], {
        color: '#2563eb',
        fillColor: '#60a5fa',
        fillOpacity: 0.4,
        radius: radKm * 1000
    }).addTo(map);

    map.fitBounds(aktuellerKreis.getBounds());

    aktuellerMarker = L.marker([lat, long], {icon: redIcon})
        .addTo(map)
        .openPopup();


    //fetch data from backend
    const url = `http://localhost:8080/stations?lat=${lat}&long=${long}&radius=${radKm}&limit=${limit}&start=${start}&end=${end}`;
    console.log("Send request to:", url);
    fetch(url)
        .then(response => response.json())
        .then(result => {
            if (result.errorMessage) {
                alert("Server Response: " + result.errorMessage);
                return;
            }

            const stationen = result.data;
         
            if (!stationen || stationen.length === 0) {
                alert("No stations found in this area.");
                return;
            }

            stationen.forEach(station => {
                if (station.latitude && station.longitude) {
                    
                    const marker = L.marker([station.latitude, station.longitude])
                        .addTo(map)
                        .bindPopup(`
                            <b>${station.name}</b><br>
                            ID: ${station.id}<br>
                            Distance: ${Math.round(station.distance)} km
                        `);
                    
                    stationMarkers.push(marker);
                }
            });
        })
        .catch(error => {
            console.error("Error during fetch:", error);
            alert("Connection to server failed. Is ‘main.go’ running?");
        });
});