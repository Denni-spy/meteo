// initialize map
var map = L.map('map').setView([51.1657, 10.4515], 5);
L.Icon.Default.imagePath = 'images/';
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
const formFields = ['latitude', 'longitude', 'radius', 'selection', 'start', 'end'];

function runSearch() {
    //reading user input
    const lat = parseFloat(document.getElementById('latitude').value);
    const long = parseFloat(document.getElementById('longitude').value);
    const radiusRaw = document.getElementById('radius').value.trim();
    const selectionRaw = document.getElementById('selection').value.trim();
    const startRaw = document.getElementById('start').value.trim();
    const endRaw = document.getElementById('end').value.trim();

    //validating user input
    if (isNaN(lat) || isNaN(long) || !radiusRaw || !selectionRaw || !startRaw || !endRaw) {
        alert("Please fill in all fields (latitude, longitude, radius, limit, start, end).");
        return;
    }

    if (!Number.isInteger(Number(radiusRaw))) {
        alert("Radius must be a whole number.");
        return;
    }
    if (!Number.isInteger(Number(selectionRaw))) {
        alert("Selection must be a whole number.");
        return;
    }
    if (!Number.isInteger(Number(startRaw))) {
        alert("Start year must be a whole number.");
        return;
    }
    if (!Number.isInteger(Number(endRaw))) {
        alert("End year must be a whole number.");
        return;
    }

    const radKm = Number(radiusRaw);
    const limit = Number(selectionRaw);
    const start = Number(startRaw);
    const end = Number(endRaw);

    if (lat < -90 || lat > 90) {
        alert("Latitude must be between -90 and 90 degrees.");
        return;
    }

    if (long < -180 || long > 180) {
        alert("Longitude must be between -180 and 180 degrees.");
        return;
    }

    if (radKm <= 0 || radKm > 100) {
        alert("The radius must be between 1 and 100 km.");
        return;
    }

    if (limit < 1 || limit > 10) {
        alert("The selection must be between 1 and 10 stations.");
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

    // Save form values so they persist across navigation
    const values = {};
    formFields.forEach(id => { values[id] = document.getElementById(id).value; });
    localStorage.setItem('meteoFormValues', JSON.stringify(values));

    // Mark that a search has been performed in this tab
    sessionStorage.setItem('meteoSearched', 'true');

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

    aktuellerMarker = L.marker([lat, long], { icon: redIcon })
        .addTo(map)
        .openPopup();


    //fetch data from backend
    const url = `/api/stations?lat=${lat}&long=${long}&radius=${radKm}&limit=${limit}&start=${start}&end=${end}`;
    console.log("Send request to:", url);
    fetch(url)
        .then(response => response.json())
        .then(result => {
            if (result.errorMessage) {
                alert(result.errorMessage);
                return;
            }

            const stationen = result.data;

            if (!stationen || stationen.length === 0) {
                return;
            }

            stationen.forEach(station => {
                if (station.latitude != null && station.longitude != null) {

                    const marker = L.marker([station.latitude, station.longitude])
                        .addTo(map)
                        .bindPopup(`
                            <b>${station.name}</b><br>
                            ID: ${station.id}<br>
                            Distance: ${Math.round(station.distance)} km
                        `)
                        .on("mouseover", function () { this.openPopup(); })
                        .on("mouseout", function () { this.closePopup(); })
                        .on("click", () => window.location.href = `detailview.html?id=${station.id}&name=${encodeURIComponent(station.name)}&start=${start}&end=${end}`);
                    stationMarkers.push(marker);
                }
            });
        })
        .catch(error => {
            console.error("Error during fetch:", error);
            alert("Connection to server failed. Is 'main.go' running?");
        });
}

form.addEventListener('submit', function (event) {
    event.preventDefault();
    runSearch();
});

// On page load: if the user already searched in this tab session, restore
// their last form values and re-run the search (handles back-button from detail view).
// On first visit (no sessionStorage flag), the HTML default values are shown as-is.
if (sessionStorage.getItem('meteoSearched')) {
    const data = localStorage.getItem('meteoFormValues');
    if (data) {
        const values = JSON.parse(data);
        formFields.forEach(id => {
            if (values[id] !== undefined) {
                document.getElementById(id).value = values[id];
            }
        });
        runSearch();
    }
}
