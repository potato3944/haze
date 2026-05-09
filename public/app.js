document.addEventListener('DOMContentLoaded', () => {
    const cityNameEl = document.getElementById('city-name');
    const refreshBtn = document.getElementById('refresh-btn');
    const tempValEl = document.getElementById('temp-value');
    const weatherDescEl = document.getElementById('weather-desc');
    const humidityEl = document.getElementById('humidity-info');
    const aqiValEl = document.getElementById('aqi-value');
    const aqiStatusEl = document.getElementById('aqi-status');
    const aqiBarEl = document.getElementById('aqi-bar');
    const healthAdviceEl = document.getElementById('health-advice');
    const actionTipEl = document.getElementById('action-tip');
    const weatherIconEl = document.getElementById('main-icon');
    const dateEl = document.getElementById('current-date');

    let weatherChart;

    // 显示日期
    const now = new Date();
    dateEl.innerText = `${now.getFullYear()}年${now.getMonth()+1}月${now.getDate()}日`;

    // 启动
    locate();

    refreshBtn.addEventListener('click', () => {
        cityNameEl.innerText = '定位中...';
        locate();
    });

    async function locate() {
        cityNameEl.innerText = '定位中...';
        actionTipEl.innerText = '正在获取位置...';

        try {
            const pos = await getPosition();
            const lat = pos.coords.latitude;
            const lon = pos.coords.longitude;

            // 把经纬度发给后端，后端返回城市+天气+AQI
            const resp = await fetch('/api/weather', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ lat, lon })
            });

            if (!resp.ok) throw new Error(await resp.text());

            const data = await resp.json();
            cityNameEl.innerText = data.city;
            updateUI(data);
            renderChart(data.forecast);
        } catch (err) {
            console.error(err);
            cityNameEl.innerText = '获取失败';
            actionTipEl.innerText = '无法获取数据: ' + err.message;
        }
    }

    function getPosition() {
        return new Promise((resolve, reject) => {
            if (!navigator.geolocation) {
                reject(new Error('浏览器不支持定位'));
                return;
            }
            navigator.geolocation.getCurrentPosition(resolve, reject, { timeout: 8000 });
        });
    }

    function updateUI(data) {
        tempValEl.innerText = data.temp;
        weatherDescEl.innerText = data.desc;
        humidityEl.innerText = `湿度: ${data.humidity}%`;
        aqiValEl.innerText = data.aqi;
        aqiStatusEl.innerText = data.status;
        healthAdviceEl.innerText = data.advice;
        actionTipEl.innerText = data.advice;

        let color = 'var(--status-good)';
        if (parseInt(data.aqi) > 100) color = 'var(--status-poor)';
        else if (parseInt(data.aqi) > 50) color = 'var(--status-moderate)';

        aqiStatusEl.style.backgroundColor = color;
        aqiBarEl.style.width = `${Math.min(parseInt(data.aqi) / 2, 100)}%`;
        aqiBarEl.style.backgroundColor = color;
        weatherIconEl.className = parseInt(data.aqi) > 100 ? 'fas fa-smog' : 'fas fa-cloud-sun';
    }

    function renderChart(forecast) {
        const ctx = document.getElementById('weatherChart').getContext('2d');
        if (weatherChart) weatherChart.destroy();

        weatherChart = new Chart(ctx, {
            type: 'line',
            data: {
                labels: forecast.map(f => f.time),
                datasets: [
                    {
                        label: '温度 (°C)',
                        data: forecast.map(f => f.temp),
                        borderColor: '#38bdf8',
                        backgroundColor: 'rgba(56, 189, 248, 0.1)',
                        fill: true, tension: 0.4
                    },
                    {
                        label: '湿度 (%)',
                        data: forecast.map(f => f.hum),
                        borderColor: '#10b981',
                        backgroundColor: 'rgba(16, 185, 129, 0.1)',
                        fill: true, tension: 0.4
                    }
                ]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: { legend: { labels: { color: '#94a3b8' } } },
                scales: {
                    y: { ticks: { color: '#94a3b8' }, grid: { color: 'rgba(255, 255, 255, 0.05)' } },
                    x: { ticks: { color: '#94a3b8' }, grid: { display: false } }
                }
            }
        });
    }
});
