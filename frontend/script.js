// initialize map
var map = L.map('map').setView([51.1657, 10.4515], 5);
L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png').addTo(map);

let aktuellerKreis = null;
let aktuellerMarker = null;

const form = document.getElementById('meteoForm');

form.addEventListener('submit', function(event) {
    event.preventDefault(); 

    const lat = parseFloat(document.getElementById('latitude').value);
    const lon = parseFloat(document.getElementById('longitude').value);
    const radKm = parseFloat(document.getElementById('radius').value);

    // validation
    if (isNaN(lat) || isNaN(lon) || isNaN(radKm)) {
        alert("Bitte fülle die Felder Breitengrad, Längengrad und Radius aus!");
        return;
    }

    if (radKm <= 0 || radKm > 20000) {
        alert("Der Radius muss zwischen 1 und 20.000 km liegen.");
        return;
    }   

    // remove circle
    if (aktuellerKreis) {
        map.removeLayer(aktuellerKreis);
    }
    
    // remove marker
    if (aktuellerMarker) {
        map.removeLayer(aktuellerMarker);
    }

    aktuellerKreis = L.circle([lat, lon], {
        color: '#2563eb',
        fillColor: '#60a5fa',
        fillOpacity: 0.4,
        radius: radKm * 1000   
    }).addTo(map);

    map.fitBounds(aktuellerKreis.getBounds());
    
    aktuellerMarker = L.marker([lat, lon])
        .addTo(map)
        .bindPopup("Zentrum")
        .openPopup();
});