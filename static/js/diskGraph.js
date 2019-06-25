var diskData = []

fetch("/data/" + escapedDiskName)
    .then((resp) => resp.json())
    .then(function (data) {
        for (i = 0; i < data.length; i++) {
            diskData.push({ x: new Date(data[i].time), y: Math.round((data[i].bytes / 1073741824) * 100) / 100 });
        }
        diskUsageGraph.update();
    });


var config = {
    type: 'line',
    data: {
        datasets: [{
            label: diskName,
            backgroundColor: "#007bff",
            borderColor: "#007bff",
            fill: false,
            data: diskData
        }]
    },
    options: {
        legend: {
            display: false
        },
        responsive: true,
        title: {
            display: false,
            text: 'Disk Space Availible at ' + diskName
        },
        scales: {
            xAxes: [{
                type: 'time',
                display: true,
                scaleLabel: {
                    display: true,
                    labelString: 'Date'
                },
                ticks: {
                    major: {
                        fontStyle: 'bold',
                        fontColor: '#FF0000'
                    }
                }
            }],
            yAxes: [{
                display: true,
                scaleLabel: {
                    display: true,
                    labelString: 'Disk Space Availible (GB)'
                }
            }]
        },
        elements: {
            line: {
                tension: 0 // disables bezier curves
            }
        }
    }
};

window.onload = function () {
    var ctx = document.getElementById('diskUsageCanvas').getContext('2d');
    diskUsageGraph = new Chart(ctx, config);
}

document.getElementById("dayGraph").addEventListener("click",function(){
    document.getElementById("dropdownMenuButton").innerHTML = "Day"
    changeGraph(1)
});

document.getElementById("weekGraph").addEventListener("click",function(){
    document.getElementById("dropdownMenuButton").innerHTML = "Week"
    changeGraph(7)
});

document.getElementById("monthGraph").addEventListener("click",function(){
    document.getElementById("dropdownMenuButton").innerHTML = "Month"
    changeGraph(30)
});

document.getElementById("yearGraph").addEventListener("click",function(){
    document.getElementById("dropdownMenuButton").innerHTML = "Year"
    changeGraph(365)
});

function changeGraph(daysAgo) {
    diskData = [];
    diskUsageGraph.data.datasets[0].data = diskData;

    sinceTime = Math.floor(Date.now() / 1000) - (3600*24*daysAgo);

    fetch("/data/" + escapedDiskName + "?since=" + sinceTime)
        .then((resp) => resp.json())
        .then(function (data) {
            for (i = 0; i < data.length; i++) {
                diskData.push({ x: new Date(data[i].time), y: Math.round((data[i].bytes / 1073741824) * 100) / 100 });
            }
            diskUsageGraph.update();
        });
}
