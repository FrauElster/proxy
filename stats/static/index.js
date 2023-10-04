
async function main() {
    const url = '/api/targets'
    const response = await fetch(url);
    const data = await response.json();


    if (!data.targets) {
        // TODO: some not found page
        return
    }

    data.targets.forEach(target => createTab(target))
    setActiveTab(data.targets[0]);
}


function createTab(target) {
    const mobileTab = document.createElement('option');
    mobileTab.id = `mobile-tab-${target}`
    mobileTab.innerText = target;
    mobileTab.onclick = () => setActiveTab(target)

    const mobileTabContainer = document.getElementById('mobile-tabs');
    mobileTabContainer.appendChild(mobileTab);

    const desktopTab = document.createElement('span');
    desktopTab.id = `desktop-tab-${target}`
    desktopTab.className = "border-transparent text-gray-500 cursor-pointer hover:border-gray-300 hover:text-gray-700 grow border-b-2 py-4 px-1 text-center text-sm font-medium";
    desktopTab.innerText = target;
    desktopTab.onclick = () => setActiveTab(target)
    const desktopTabContainer = document.getElementById('desktop-tabs');
    desktopTabContainer.appendChild(desktopTab);
}

function setActiveTab(target) {
    console.log(`active target: ${target}`)
    const allMobileTabs = document.getElementById('mobile-tabs').children;
    for (let tab of allMobileTabs) {
        tab.selected = false
    }
    const mobileTab = document.getElementById(`mobile-tab-${target}`);
    mobileTab.selected = true;

    const allDesktopTabs = document.getElementById('desktop-tabs').children;
    for (let tab of allDesktopTabs) {
        tab.className = "border-transparent text-gray-500 cursor-pointer hover:border-gray-300 hover:text-gray-700 grow border-b-2 py-4 px-1 text-center text-sm font-medium";
    }
    const desktopTab = document.getElementById(`desktop-tab-${target}`);
    desktopTab.className = "border-sky-500 cursor-pointer grow text-sky-600 border-b-2 py-4 px-1 text-center text-sm font-medium active";
    
    loadData(target);
}
    
async function loadData(target) {
    try{
        fetchDataAndUpdate(target);
        const interval = setInterval(async () => {
            const activeTarget = document.getElementsByClassName("active")[0].innerHTML
            if (activeTarget !== target) clearInterval(interval);

            await fetchDataAndUpdate(target);
        }, 5000);
    } catch (error) {
        console.error(`Error fetching data for ${target}: ${error}`);
    }
}

async function fetchDataAndUpdate(target) {
    try {
        const response = await fetch(`/api/targets/${target}`);
        const data = await response.json();

        document.getElementById("capture-window").innerText = `Capture Window ${formatDuration(data.windowDuration)}`
        document.getElementById("total-requests").innerText = data.totalRequestCount;
        document.getElementById("stat-start-date").innerText = `since ${formatRFC3999Timestamp(data.statStartDate)}`;
        document.getElementById("request-count").innerText = data.requestCount;
        document.getElementById("total-response-time").innerText = formatDuration(data.totalAvgResponseTime);
        document.getElementById("response-time").innerText = formatDuration(data.avgResponseTime);
        document.getElementById("request-rate").innerText = data.requestRate.toFixed(2);

        const errorRate = document.getElementById("error-rate")
        errorRate.innerText = data.errorRate ? data.errorRate.toFixed(2) : "0";
        data.errorRate > 0 ? errorRate.classList.add("text-red-500") : errorRate.classList.remove("text-red-500");
    } catch (error) {
        console.error(`Error fetching data for ${target}: ${error}`);
    }
}


function formatDuration(durationInNanoseconds) {
    // Define constants for conversion
    const nanosecondsInSecond = 1e9;
    const nanosecondsInMillisecond = 1e6;
  
    // Calculate the components of the duration
    const hours = Math.floor(durationInNanoseconds / (nanosecondsInSecond * 60 * 60));
    const minutes = Math.floor((durationInNanoseconds % (nanosecondsInSecond * 60 * 60)) / (nanosecondsInSecond * 60));
    const seconds = Math.floor((durationInNanoseconds % (nanosecondsInSecond * 60)) / nanosecondsInSecond);
    const milliseconds = Math.floor((durationInNanoseconds % nanosecondsInSecond) / nanosecondsInMillisecond);
  
    // Create an array to store the components
    const components = [];
  
    // Add components with values greater than 0
    if (hours > 0) components.push(`${hours} hours`);
    if (minutes > 0) components.push(`${minutes} minutes`);
    if (seconds > 0) components.push(`${seconds} seconds`);
    if (milliseconds > 0) components.push(`${milliseconds} milliseconds`);
  
    // Join the components into a single string
    return components.join(', ');
  }
  
  function formatRFC3999Timestamp(rfc3999Timestamp) {
    // Convert the RFC3999 timestamp to milliseconds
    const milliseconds = Date.parse(rfc3999Timestamp);
  
    // Check if the conversion was successful
    if (isNaN(milliseconds)) {
      return 'Invalid Timestamp';
    }
  
    // Create a Date object with the milliseconds timestamp
    const date = new Date(milliseconds);
  
    // Get day, month, and year components
    const day = String(date.getDate()).padStart(2, '0');
    const month = String(date.getMonth() + 1).padStart(2, '0'); // Month is 0-based, so add 1
    const year = date.getFullYear();
  
    // Format as "dd.mm.yyyy"
    return `${day}.${month}.${year}`;
  }

document.addEventListener('DOMContentLoaded', main);