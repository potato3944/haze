package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// ========== 返回给前端的结构 ==========

type WeatherResult struct {
	City     string     `json:"city"`
	Temp     string     `json:"temp"`
	Desc     string     `json:"desc"`
	Humidity string     `json:"humidity"`
	AQI      string     `json:"aqi"`
	PM25     string     `json:"pm25"`
	PM10     string     `json:"pm10"`
	Status   string     `json:"status"`
	Advice   string     `json:"advice"`
	Forecast []Forecast `json:"forecast"`
}

type Forecast struct {
	Time string `json:"time"`
	Temp int    `json:"temp"`
	Hum  int    `json:"hum"`
}

// ========== Open-Meteo API 响应结构 ==========

type ForecastResult struct {
	Current struct {
		Temperature float64 `json:"temperature_2m"`
		Humidity    float64 `json:"relative_humidity_2m"`
		WeatherCode int     `json:"weather_code"`
	} `json:"current"`
	Hourly struct {
		Time        []string  `json:"time"`
		Temperature []float64 `json:"temperature_2m"`
		Humidity    []float64 `json:"relative_humidity_2m"`
	} `json:"hourly"`
}

type AirQualityResult struct {
	Current struct {
		USAQI float64 `json:"us_aqi"`
		PM25  float64 `json:"pm2_5"`
		PM10  float64 `json:"pm10"`
	} `json:"current"`
}

type NominatimResult struct {
	DisplayName string `json:"display_name"`
	Address     struct {
		City         string `json:"city"`
		Town         string `json:"town"`
		Village      string `json:"village"`
		Suburb       string `json:"suburb"`
		Municipality string `json:"municipality"`
		County       string `json:"county"`
		State        string `json:"state"`
	} `json:"address"`
}

// ========== WMO 天气代码映射 ==========

var weatherCodeMap = map[int]string{
	0: "晴", 1: "大部晴朗", 2: "多云", 3: "阴天",
	45: "雾", 48: "雾凇", 51: "小毛毛雨", 53: "毛毛雨",
	55: "大毛毛雨", 61: "小雨", 63: "中雨", 65: "大雨",
	71: "小雪", 73: "中雪", 75: "大雪", 80: "小阵雨",
	81: "中阵雨", 82: "大阵雨", 95: "雷暴",
}

func main() {
	fs := http.FileServer(http.Dir("./public"))
	http.Handle("/", fs)
	http.HandleFunc("/api/weather", handleWeather)

	fmt.Println("服务器启动于 http://localhost:80")
	log.Fatal(http.ListenAndServe(":80", nil))
}

func handleWeather(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "仅支持 POST", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Lat float64 `json:"lat"`
		Lon float64 `json:"lon"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("收到坐标: %.4f, %.4f", req.Lat, req.Lon)

	data, err := fetchWeather(req.Lat, req.Lon)
	if err != nil {
		log.Printf("获取天气失败: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func fetchWeather(lat, lon float64) (*WeatherResult, error) {
	// 并行请求三个 API
	type weatherCh struct {
		data *ForecastResult
		err  error
	}
	type aqiCh struct {
		aqi, pm25, pm10 float64
		err             error
	}
	type geoCh struct {
		city string
		err  error
	}

	wCh := make(chan weatherCh, 1)
	aCh := make(chan aqiCh, 1)
	gCh := make(chan geoCh, 1)

	go func() {
		f, err := getWeatherForecast(lat, lon)
		wCh <- weatherCh{f, err}
	}()
	go func() {
		aqi, pm25, pm10, err := getAirQuality(lat, lon)
		aCh <- aqiCh{aqi, pm25, pm10, err}
	}()
	go func() {
		city, err := reverseGeocode(lat, lon)
		gCh <- geoCh{city, err}
	}()

	wRes := <-wCh
	aRes := <-aCh
	gRes := <-gCh

	if wRes.err != nil {
		return nil, fmt.Errorf("天气: %w", wRes.err)
	}
	if aRes.err != nil {
		return nil, fmt.Errorf("空气质量: %w", aRes.err)
	}

	city := "未知城市"
	if gRes.err == nil {
		city = gRes.city
	} else {
		log.Printf("地理逆编码失败: %v", gRes.err)
	}
	log.Printf("  城市: %s", city)

	forecast := wRes.data
	desc := weatherCodeMap[forecast.Current.WeatherCode]
	if desc == "" {
		desc = "未知"
	}

	aqi := int(aRes.aqi)
	status, advice := aqiLevel(aqi)

	var forecastData []Forecast
	for i := 0; i < len(forecast.Hourly.Time) && i < 24; i += 4 {
		t := forecast.Hourly.Time[i]
		if len(t) >= 16 {
			t = t[11:16]
		}
		forecastData = append(forecastData, Forecast{
			Time: t,
			Temp: int(forecast.Hourly.Temperature[i]),
			Hum:  int(forecast.Hourly.Humidity[i]),
		})
	}

	return &WeatherResult{
		City:     city,
		Temp:     strconv.Itoa(int(forecast.Current.Temperature)),
		Desc:     desc,
		Humidity: strconv.Itoa(int(forecast.Current.Humidity)),
		AQI:      strconv.Itoa(aqi),
		PM25:     fmt.Sprintf("%.1f", aRes.pm25),
		PM10:     fmt.Sprintf("%.1f", aRes.pm10),
		Status:   status,
		Advice:   advice,
		Forecast: forecastData,
	}, nil
}

func reverseGeocode(lat, lon float64) (string, error) {
	apiURL := fmt.Sprintf(
		"https://nominatim.openstreetmap.org/reverse?lat=%.4f&lon=%.4f&format=json&accept-language=zh&zoom=10",
		lat, lon,
	)
	body, err := httpGet(apiURL)
	if err != nil {
		return "", err
	}

	var result NominatimResult
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	addr := result.Address

	// 从 display_name 中提取地级市（以"市"结尾的部分），如"西安市"
	prefCity := extractPrefCity(result.DisplayName)

	// 获取区/县级名称
	district := ""
	switch {
	case addr.City != "":
		district = addr.City
	case addr.Town != "":
		district = addr.Town
	case addr.Village != "":
		district = addr.Village
	case addr.Suburb != "":
		district = addr.Suburb
	case addr.Municipality != "":
		district = addr.Municipality
	case addr.County != "":
		district = addr.County
	}

	// 组合：地级市 + 区县，去重（避免出现"西安市西安市"）
	if prefCity != "" && district != "" && prefCity != district {
		return prefCity + district, nil
	}
	if district != "" {
		return district, nil
	}
	if addr.State != "" {
		return addr.State, nil
	}
	return "未知城市", nil
}

// extractPrefCity 从 Nominatim 的 display_name 中提取地级市名称（以"市"结尾的部分）
// 例："兴隆街道, 长安区, 西安市, 陕西省, 中国" -> "西安市"
func extractPrefCity(displayName string) string {
	parts := strings.Split(displayName, ", ")
	for _, p := range parts {
		if strings.HasSuffix(p, "市") {
			return p
		}
	}
	return ""
}

func getWeatherForecast(lat, lon float64) (*ForecastResult, error) {
	apiURL := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f"+
			"&current=temperature_2m,relative_humidity_2m,weather_code"+
			"&hourly=temperature_2m,relative_humidity_2m"+
			"&forecast_days=1&timezone=auto",
		lat, lon,
	)
	body, err := httpGet(apiURL)
	if err != nil {
		return nil, err
	}

	var result ForecastResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func getAirQuality(lat, lon float64) (float64, float64, float64, error) {
	apiURL := fmt.Sprintf(
		"https://air-quality-api.open-meteo.com/v1/air-quality?latitude=%.4f&longitude=%.4f"+
			"&current=us_aqi,pm2_5,pm10&timezone=auto",
		lat, lon,
	)
	body, err := httpGet(apiURL)
	if err != nil {
		return 0, 0, 0, err
	}

	var result AirQualityResult
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, 0, 0, err
	}
	return result.Current.USAQI, result.Current.PM25, result.Current.PM10, nil
}

func aqiLevel(aqi int) (string, string) {
	switch {
	case aqi <= 50:
		return "优", "空气清新，非常适合户外活动。"
	case aqi <= 100:
		return "良", "空气质量尚可，极少数敏感人群应减少户外活动。"
	case aqi <= 150:
		return "轻度污染", "敏感人群应减少户外活动，建议佩戴口罩。"
	case aqi <= 200:
		return "中度污染", "减少户外活动，外出请佩戴口罩。"
	case aqi <= 300:
		return "重度污染", "避免户外活动，外出必须佩戴防护口罩。"
	default:
		return "严重污染", "严禁户外活动，请紧闭门窗。"
	}
}

func httpGet(url string) ([]byte, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Nominatim 需要合法的 User-Agent，自定义 App UA 会被拒绝，使用浏览器 UA 即可
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP 错误: %d, body: %s", resp.StatusCode, string(body))
	}
	return io.ReadAll(resp.Body)
}
