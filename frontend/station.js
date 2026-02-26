const urlParams = new URLSearchParams(window.location.search);
const id = urlParams.get('id');
const stationName = urlParams.get('name');
const startYear = urlParams.get('start') ? parseInt(urlParams.get('start')) : null;
const endYear = urlParams.get('end') ? parseInt(urlParams.get('end')) : null;
const url = `/api/station?id=${id}`;
const spinner = document.getElementById('loading-spinner');

// Display station name if provided
const nameEl = document.getElementById('station-name');
if (nameEl && stationName) {
    nameEl.textContent = `Station: ${stationName}`;
}

let chartInstance = null;
let cachedAnnualData = null;
let cachedSeasonalData = null;
let showSeasons = false;

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

// Filter data to only include years within the user-specified range.
function filterByYearRange(data, start, end) {
    if (!data) return data;
    return data.filter(row => {
        if (start !== null && row.year < start) return false;
        if (end !== null && row.year > end) return false;
        return true;
    });
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

        const annualDataFull = fillGaps(result.data.annual);
        const minYear = annualDataFull.length > 0 ? annualDataFull[0].year : 0;
        const maxYear = annualDataFull.length > 0 ? annualDataFull[annualDataFull.length - 1].year : 0;
        const seasonalDataFull = fillSeasonalGaps(result.data.seasonal, minYear, maxYear);

        // Filter to user-specified year range
        const annualData = filterByYearRange(annualDataFull, startYear, endYear);
        const seasonalData = filterByYearRange(seasonalDataFull, startYear, endYear);

        cachedAnnualData = annualData;
        cachedSeasonalData = seasonalData;
        draw(annualData, seasonalData, showSeasons);
        fillTable(seasonalData, annualData);
    })
    .catch(error => {
        spinner.style.display = 'none';
        console.error("Error during fetch:", error);
        alert("Connection to server failed. Is 'main.go' running?");
    });

function draw(annualData, seasonalData, withSeasons) {
    const colors = getChartColors();

    if (chartInstance) {
        chartInstance.destroy();
    }

    const datasets = [];

    // Only show annual lines when seasons are off
    if (!withSeasons) {
        datasets.push(
            {
                label: 'Tmin (annual)',
                data: annualData.map(row => row.tmin),
                borderColor: colors.tmin,
                backgroundColor: colors.tmin,
                spanGaps: false,
                borderWidth: 2,
            },
            {
                label: 'Tmax (annual)',
                data: annualData.map(row => row.tmax),
                borderColor: colors.tmax,
                backgroundColor: colors.tmax,
                spanGaps: false,
                borderWidth: 2,
            }
        );
    }

    if (withSeasons && seasonalData) {
        // Build a map: year -> { season -> { tmin, tmax } }
        const seasonMap = new Map();
        for (const row of seasonalData) {
            if (!seasonMap.has(row.year)) seasonMap.set(row.year, {});
            seasonMap.get(row.year)[row.season] = row;
        }

        const seasonDefs = [
            { season: 'Winter', minColor: '#6bb7e0', maxColor: '#e07c6b' },
            { season: 'Spring', minColor: '#4caf50', maxColor: '#ff9800' },
            { season: 'Summer', minColor: '#00bcd4', maxColor: '#e91e63' },
            { season: 'Autumn', minColor: '#9c7ae6', maxColor: '#c77a28' },
        ];

        for (const def of seasonDefs) {
            datasets.push({
                label: `${def.season} Tmin`,
                data: annualData.map(row => {
                    const s = seasonMap.get(row.year);
                    return s && s[def.season] ? s[def.season].tmin : null;
                }),
                borderColor: def.minColor,
                backgroundColor: def.minColor,
                spanGaps: false,
                borderWidth: 1.5,
            });
            datasets.push({
                label: `${def.season} Tmax`,
                data: annualData.map(row => {
                    const s = seasonMap.get(row.year);
                    return s && s[def.season] ? s[def.season].tmax : null;
                }),
                borderColor: def.maxColor,
                backgroundColor: def.maxColor,
                spanGaps: false,
                borderWidth: 1.5,
            });
        }
    }

    chartInstance = new Chart(
        document.getElementById('station'),
        {
            type: 'line',
            data: {
                labels: annualData.map(row => row.year),
                datasets: datasets
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                scales: {
                    x: {
                        ticks: { color: colors.text },
                        grid: { color: colors.grid },
                        title: {
                            display: true,
                            text: 'Year',
                            color: colors.text,
                        },
                    },
                    y: {
                        ticks: { color: colors.text },
                        grid: { color: colors.grid },
                        title: {
                            display: true,
                            text: 'Temperature (°C)',
                            color: colors.text,
                        },
                    }
                },
                plugins: {
                    legend: {
                        display: true,
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
const seasonsWrapper = document.getElementById('seasons-toggle')?.closest('.dark-mode-toggle-wrapper');

diagramBtn.addEventListener("click", () => {
    diagramBtn.classList.add("active")
    tableBtn.classList.remove("active")
    document.getElementById("station").classList.remove("hidden")
    document.getElementById("table-container").classList.add("hidden")
    if (seasonsWrapper) seasonsWrapper.classList.remove("hidden");
})

tableBtn.addEventListener("click", () => {
    tableBtn.classList.add("active")
    diagramBtn.classList.remove("active")
    document.getElementById("station").classList.add("hidden")
    document.getElementById("table-container").classList.remove("hidden")
    if (seasonsWrapper) seasonsWrapper.classList.add("hidden");
})

// Redraw chart when dark mode changes
const darkToggle = document.getElementById('dark-mode-toggle');
if (darkToggle) {
    darkToggle.addEventListener('change', function () {
        if (cachedAnnualData) {
            setTimeout(() => draw(cachedAnnualData, cachedSeasonalData, showSeasons), 50);
        }
    });
}

// Toggle seasonal data lines on the chart
const seasonsToggle = document.getElementById('seasons-toggle');
if (seasonsToggle) {
    seasonsToggle.addEventListener('change', function () {
        showSeasons = seasonsToggle.checked;
        if (cachedAnnualData) {
            draw(cachedAnnualData, cachedSeasonalData, showSeasons);
        }
    });
}
