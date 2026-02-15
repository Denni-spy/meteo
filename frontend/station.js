const urlParams = new URLSearchParams(window.location.search);
const id = urlParams.get('id');
const stationName = urlParams.get('name');
const url = `/api/station?id=${id}`;
const spinner = document.getElementById('loading-spinner');

// Display station name if provided
const nameEl = document.getElementById('station-name');
if (nameEl && stationName) {
    nameEl.textContent = `Stationname: ${stationName}`;
}

let chartInstance = null;
let cachedAnnualData = null;

function getChartColors() {
    const style = getComputedStyle(document.documentElement);
    return {
        tmin: style.getPropertyValue('--tmin-color').trim(),
        tmax: style.getPropertyValue('--tmax-color').trim(),
        text: style.getPropertyValue('--text-color').trim(),
        grid: style.getPropertyValue('--border-color').trim(),
    };
}

// Fill gaps in annual data so every year from min to max is present.
// Missing years get { year, tmin: null, tmax: null }.
function fillGaps(data) {
    if (!data || data.length === 0) return data;

    const byYear = new Map();
    for (const row of data) {
        byYear.set(row.year, row);
    }

    const years = data.map(r => r.year);
    const minYear = Math.min(...years);
    const maxYear = Math.max(...years);

    const filled = [];
    for (let y = minYear; y <= maxYear; y++) {
        if (byYear.has(y)) {
            filled.push(byYear.get(y));
        } else {
            filled.push({ year: y, tmin: null, tmax: null });
        }
    }
    return filled;
}

// Fill gaps in seasonal data so every year from min to max has all 4 seasons.
// Missing season entries get { year, season, tmin: null, tmax: null }.
function fillSeasonalGaps(seasonalData, minYear, maxYear) {
    if (!seasonalData || seasonalData.length === 0) return seasonalData;

    const seasons = ["Winter", "Spring", "Summer", "Autumn"];
    const key = (year, season) => `${year}-${season}`;

    const byKey = new Map();
    for (const row of seasonalData) {
        byKey.set(key(row.year, row.season), row);
    }

    const filled = [];
    for (let y = minYear; y <= maxYear; y++) {
        for (const s of seasons) {
            const k = key(y, s);
            if (byKey.has(k)) {
                filled.push(byKey.get(k));
            } else {
                filled.push({ year: y, season: s, tmin: null, tmax: null });
            }
        }
    }
    return filled;
}

console.log("Send request to:", url);
spinner.style.display = 'block';

fetch(url)
    .then(response => response.json())
    .then(result => {
        spinner.style.display = 'none'
        if (result.errorMessage) {
            alert("Server Response: " + result.errorMessage);
            return;
        }

        const annualData = fillGaps(result.data.annual);
        const minYear = annualData.length > 0 ? annualData[0].year : 0;
        const maxYear = annualData.length > 0 ? annualData[annualData.length - 1].year : 0;
        const seasonalData = fillSeasonalGaps(result.data.seasonal, minYear, maxYear);

        cachedAnnualData = annualData;
        draw(annualData);
        fillTable(seasonalData, annualData);
    })
    .catch(error => {
        spinner.style.display = 'none';
        console.error("Error during fetch:", error);
        alert("Connection to server failed. Is 'main.go' running?");
    });

function draw(data) {
    const colors = getChartColors();

    if (chartInstance) {
        chartInstance.destroy();
    }

    chartInstance = new Chart(
        document.getElementById('station'),
        {
            type: 'line',
            data: {
                labels: data.map(row => row.year),
                datasets: [
                    {
                        label: 'tmin per year',
                        data: data.map(row => row.tmin),
                        borderColor: colors.tmin,
                        backgroundColor: colors.tmin,
                        spanGaps: false,
                    },
                    {
                        label: 'tmax per year',
                        data: data.map(row => row.tmax),
                        borderColor: colors.tmax,
                        backgroundColor: colors.tmax,
                        spanGaps: false,
                    }
                ]
            },
            options: {
                responsive: true,
                maintainAspectRatio: true,
                scales: {
                    x: {
                        ticks: { color: colors.text },
                        grid: { color: colors.grid },
                    },
                    y: {
                        ticks: { color: colors.text },
                        grid: { color: colors.grid },
                    }
                },
                plugins: {
                    legend: {
                        labels: { color: colors.text }
                    }
                }
            }
        }
    );
}

function fillTable(seasonalData, annualData) {
    const yearMap = new Map();
    for (const e of annualData) {
        yearMap.set(e.year, { tmin: e.tmin, tmax: e.tmax });
    }

    const bodyData = document.getElementById('tbody');

    // Build a map: year -> { tmin, tmax, winterMin, winterMax, ... }
    const tableData = new Map();

    // Seed every year from annualData (which is already gap-filled)
    for (const e of annualData) {
        tableData.set(e.year, { tmin: e.tmin, tmax: e.tmax });
    }

    // Merge seasonal data
    for (const e of seasonalData) {
        let obj = tableData.get(e.year);
        if (!obj) {
            const yearData = yearMap.get(e.year);
            obj = { tmin: yearData?.tmin ?? null, tmax: yearData?.tmax ?? null };
            tableData.set(e.year, obj);
        }

        if (e.season === "Winter") {
            obj.winterMin = e.tmin;
            obj.winterMax = e.tmax;
        } else if (e.season === "Spring") {
            obj.springMin = e.tmin;
            obj.springMax = e.tmax;
        } else if (e.season === "Summer") {
            obj.summerMin = e.tmin;
            obj.summerMax = e.tmax;
        } else if (e.season === "Autumn") {
            obj.autumnMin = e.tmin;
            obj.autumnMax = e.tmax;
        }
    }

    // Sort by year and render rows
    const sortedYears = [...tableData.keys()].sort((a, b) => a - b);
    for (const year of sortedYears) {
        const data = tableData.get(year);
        const val = (v) => (v !== undefined && v !== null) ? v : '-';

        const tr = document.createElement("tr");
        tr.innerHTML = `
        <td><b>${year}</b></td>
        
        <td class="cell-tmin">${val(data.tmin)}</td>
        <td class="cell-tmax">${val(data.tmax)}</td>

        <td>${val(data.winterMin)}</td>
        <td>${val(data.winterMax)}</td>
        <td>${val(data.springMin)}</td>
        <td>${val(data.springMax)}</td>
        <td>${val(data.summerMin)}</td>
        <td>${val(data.summerMax)}</td>
        <td>${val(data.autumnMin)}</td>
        <td>${val(data.autumnMax)}</td>
    `;
        bodyData.appendChild(tr);
    }
}

const diagramBtn = document.getElementById("btn-chart");
const tableBtn = document.getElementById("btn-table");

diagramBtn.addEventListener("click", () => {
    diagramBtn.classList.add("active")
    tableBtn.classList.remove("active")
    document.getElementById("station").classList.remove("hidden")
    document.getElementById("table-container").classList.add("hidden")

})

tableBtn.addEventListener("click", () => {
    tableBtn.classList.add("active")
    diagramBtn.classList.remove("active")
    document.getElementById("station").classList.add("hidden")
    document.getElementById("table-container").classList.remove("hidden")

})

// Redraw chart when dark mode changes
const darkToggle = document.getElementById('dark-mode-toggle');
if (darkToggle) {
    darkToggle.addEventListener('change', function () {
        if (cachedAnnualData) {
            // Small delay to let CSS variables update
            setTimeout(() => draw(cachedAnnualData), 50);
        }
    });
}
