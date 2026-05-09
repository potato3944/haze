package main

import (
	"fmt"
	"io"
	"net/http"
)

func main() {
	lat, lon := 39.9042, 116.4074 // Beijing
	apiURL := fmt.Sprintf(
		"https://nominatim.openstreetmap.org/reverse?lat=%.4f&lon=%.4f&format=json&accept-language=zh&zoom=10",
		lat, lon,
	)
	
	client := &http.Client{}
	req, _ := http.NewRequest("GET", apiURL, nil)
	// Nominatim requires a User-Agent
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))
}
