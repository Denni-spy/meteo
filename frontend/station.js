const urlParams = new URLSearchParams(window.location.search);
const id = urlParams.get('id')
const url = `http://localhost:8080/station?id=${id}`;
const spinner = document.getElementById('loading-spinner');
console.log("Send request to:", url);
let data = []
spinner.style.display = 'block'; 
fetch(url)
    .then(response => response.json())
    .then(result => {
        spinner.style.display = 'none'
        if (result.errorMessage) {
            alert("Server Response: " + result.errorMessage);
            return;
        }
        data = result.data.annual
        draw(data)
    })
    .catch(error => {
        spinner.style.display = 'none'; 
        console.error("Error during fetch:", error);
        alert("Connection to server failed. Is ‘main.go’ running?");
    });

function draw(data) {
    new Chart(
        document.getElementById('station'),
        {
            type: 'line',
            data: {
                labels: data.map(row => row.year),
                datasets: [
                    {
                        label: 'tmin per year',
                        data: data.map(row => row.tmin)
                    },
                    {
                        label: 'tmax per year',
                        data: data.map(row => row.tmax)
                    }
                ]

            }
        }
    );
}



