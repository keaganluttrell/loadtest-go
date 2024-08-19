package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	_ "net/http/pprof"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

func writeFile(str string) {
	f, err := os.Create("ui/test.json")
	if err != nil {
		fmt.Println(err)
		return
	}
	l, err := f.WriteString(str)
	if err != nil {
		fmt.Println(err)
		f.Close()
		return
	}
	fmt.Println(l, "bytes written successfully")
	err = f.Close()
	if err != nil {
		fmt.Println(err)
		return
	}
}

func sleep(timeInSeconds int) {
	// fmt.Printf("Sleeping %d seconds...\n", timeInSeconds)
	time.Sleep(time.Duration(timeInSeconds) * time.Second)
}

type Metric struct {
	UrlString      string
	User           string
	Timestamp      time.Time
	ResponseTimeMs int64
	ResponseCode   int
	Redirect       bool
	RedirectUrl    string
}

func (m Metric) Print() {
	fmt.Printf("\tURL: %s\n", m.UrlString)
	fmt.Printf("\tUser: %s\n", m.User)
	fmt.Printf("\tTimestamp: %s\n", m.Timestamp.Format(time.RFC3339))
	fmt.Printf("\tResponse Time (ms): %d\n", m.ResponseTimeMs)
	fmt.Printf("\tResponse Code: %d\n", m.ResponseCode)
	fmt.Printf("\tRedirect: %v\n", m.Redirect)
	if m.Redirect {
		fmt.Printf("\tRedirectUrl: %v\n", m.RedirectUrl)
	}
	fmt.Printf("\t--END--\n")
}

type PlaybookMetric struct {
	PlaybookName        string
	User                string
	FailedToAuth        bool
	TotalResponseTimeMs int64
	AvgResponseTimeMs   int64
	TotalRequests       int
	TotalPlaybookTime   int64
	Metrics             []Metric
}

func (pm PlaybookMetric) Print() {
	fmt.Printf("User: %s\n", pm.User)
	fmt.Printf("PlaybookName: %s\n", pm.PlaybookName)
	fmt.Printf("FailedToAuth: %v\n", pm.FailedToAuth)
	fmt.Println("Metrics:")
	fmt.Println("Total", len(pm.Metrics))
	for _, metric := range pm.Metrics {
		metric.Print()
	}
}

func (pm *PlaybookMetric) GetResponseTimeMetrics() {
	var total int64 = 0
	for i := 0; i < len(pm.Metrics); i++ {
		total += pm.Metrics[i].ResponseTimeMs
	}
	pm.TotalResponseTimeMs = total
	pm.AvgResponseTimeMs = total / int64(len(pm.Metrics))
	pm.TotalRequests = len(pm.Metrics)
}

type LoadTestMetrics struct {
	Timestamp         time.Time
	AvgResponseTime   int64
	TotalLoadTestTime int64
	TotalResponses    int
	TotalRedirects    int
	TotalAuthFailures int
	Total500s         int
	Total400s         int
	Total200s         int
	PlaybookMetrics   []PlaybookMetric
}

func (lt *LoadTestMetrics) GetLoadTestMetrics() {

	var totalResponseTimeMs int64 = 0
	var totalResponses int64 = 0
	totalRedirects := 0
	totalAuthFailures := 0
	total200s := 0
	total400s := 0
	total500s := 0

	for i := 0; i < len(lt.PlaybookMetrics); i++ {
		if lt.PlaybookMetrics[i].FailedToAuth {
			totalAuthFailures++
		}
		responseMetrics := lt.PlaybookMetrics[i].Metrics

		for j := 0; j < len(responseMetrics); j++ {
			metric := responseMetrics[j]

			totalResponseTimeMs += metric.ResponseTimeMs
			totalResponses++

			if metric.Redirect {
				totalRedirects++
			}

			resCode := metric.ResponseCode
			if resCode >= 200 && resCode < 300 {
				total200s++
			} else if resCode >= 300 && resCode < 400 {
				totalRedirects++
			} else if resCode >= 400 && resCode < 500 {
				total400s++
			} else if resCode >= 500 && resCode < 600 {
				total500s++
			} else {
				fmt.Print("GetLoadTestMetrics -- Unknown Response Code:", resCode)
			}
		}
	}

	lt.Total200s = total200s
	lt.Total400s = total400s
	lt.Total500s = total500s
	lt.TotalRedirects = totalRedirects
	lt.TotalResponses = int(totalResponses)
	lt.AvgResponseTime = totalResponseTimeMs / totalResponses
}

func makeClient() *http.Client {
	jar, err := cookiejar.New(nil)
	if err != nil {
		log.Fatalf("Failed to create cookie jar: %v", err)
	}

	client := &http.Client{
		Jar: jar,
	}

	return client
}

func request(client *http.Client, user string, urlString string, metrics *[]Metric, loginRequest bool) bool {
	var req *http.Request
	var err error

	if loginRequest {
		loginData := url.Values{}
		loginData.Set("username", user)
		loginData.Set("password", "password")

		req, err = http.NewRequest("POST", urlString, strings.NewReader(loginData.Encode()))
		if err != nil {
			log.Fatalf("Failed to create login request: %v", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req, err = http.NewRequest("GET", urlString, http.NoBody)
		if err != nil {
			log.Fatalf("Failed to create login request: %v", err)
		}
	}

	loginUrl := "https://friends.staging.abenity.com/discounts/login"
	wasRedirectedToLogin := false
	wasRedirected := false
	var redirectUrl string = ""

	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		baseURL := fmt.Sprintf("%s://%s%s", req.URL.Scheme, req.URL.Host, req.URL.Path)
		redirectUrl = req.URL.String()
		wasRedirected = true
		if baseURL == loginUrl {
			wasRedirectedToLogin = true
		}
		if len(via) >= 10 {
			return http.ErrUseLastResponse
		}
		return nil
	}

	reqStart := time.Now()
	loginResp, err := client.Do(req)
	reqStop := time.Now()
	if err != nil {
		panic(err)
	}

	metric := Metric{
		User:           user,
		UrlString:      urlString,
		Timestamp:      time.Now(),
		ResponseTimeMs: reqStop.Sub(reqStart).Milliseconds(),
		ResponseCode:   loginResp.StatusCode,
		Redirect:       wasRedirected,
		RedirectUrl:    redirectUrl,
	}

	*metrics = append(*metrics, metric)

	return wasRedirectedToLogin
}

func playbook(wg *sync.WaitGroup, user string, urlStrings []string, playbookMetrics *[]PlaybookMetric) {
	defer wg.Done()
	fmt.Println("Playbook Started for:", user)
	loggedIn := false
	loginUrl := "https://friends.staging.abenity.com/discounts/login"

	playbookMetric := PlaybookMetric{
		User:         user,
		PlaybookName: "test",
		Metrics:      []Metric{},
		FailedToAuth: false,
	}

	client := makeClient()
	totalPlaybookTimeStart := time.Now()

	for i := 0; i < len(urlStrings); i++ {
		urlString := urlStrings[i]

		redirectedToLogin := request(client, user, urlString, &playbookMetric.Metrics, false)

		if redirectedToLogin && !loggedIn {
			randomNumber := rand.Intn(RANDMAX-RANDMIN+1) + RANDMIN
			sleep(randomNumber)
			request(client, user, loginUrl, &playbookMetric.Metrics, true)
			loggedIn = true
		} else if redirectedToLogin {
			playbookMetric.FailedToAuth = true
			break
		}
		randomNumber := rand.Intn(RANDMAX-RANDMIN+1) + RANDMIN
		sleep(randomNumber)
	}

	totalPlaybookTimeEnd := time.Now()
	playbookMetric.TotalPlaybookTime = int64(totalPlaybookTimeEnd.Sub(totalPlaybookTimeStart).Seconds())
	playbookMetric.GetResponseTimeMetrics()
	*playbookMetrics = append(*playbookMetrics, playbookMetric)
	fmt.Println("Playbook Ended for:", user)
}

func generateRandomUrls(urls []string) []string {
	shuffled := make([]string, len(urls))
	copy(shuffled, urls)
	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	randomLength := rand.Intn(len(shuffled)) + 1
	if randomLength < 3 {
		randomLength = 3
	}
	return shuffled[:randomLength]
}

const RANDMIN int = 1
const RANDMAX int = 1

// make php fpm metrics script to run during load test session
// consider a log based metric system.
// preface the log with a data type
// write all logs to stdout or a file
// use a query language to parse the log file
// get load test metrics
// get playbook metrics
func main() {
	var wg sync.WaitGroup
	commonUrls := []string{
		"https://friends.staging.abenity.com/discounts/profile/location",
		"https://friends.staging.abenity.com/discounts/profile/location",
		"https://friends.staging.abenity.com/movies",
		"https://friends.staging.abenity.com/discounts/category/Automotive",
		"https://friends.staging.abenity.com/discounts/new",
		"https://friends.staging.abenity.com/discounts/category/Apparel_and_Accessories",
		"https://friends.staging.abenity.com/discounts/category/Education",
		"https://friends.staging.abenity.com/discounts/category/Electronics",
		"https://friends.staging.abenity.com/discounts/category/Concerts_and_Events",
		"https://friends.staging.abenity.com/discounts/category/Recreation_and_Entertainment",
		"https://friends.staging.abenity.com/discounts/recommendation",
		"https://friends.staging.abenity.com/discounts/recommendation/nearby",
	}

	var loadTest LoadTestMetrics
	loadTestStart := time.Now()
	loadTest.Timestamp = loadTestStart
	total := 15
	volley := 3

	for i := 0; i < total; i++ {
		if i%volley == 0 && i != 0 {
			fi := float64(i)
			ft := float64(total)
			fmt.Printf("Round 1: %d%s\n", int64((fi/ft)*100.0), "%")
			sleep(20)
		}
		user := fmt.Sprintf("virtual_user_%d", i)
		wg.Add(1)
		go playbook(&wg, user, generateRandomUrls(commonUrls), &loadTest.PlaybookMetrics)
	}
	wg.Wait()

	loadTestEnd := time.Now()
	loadTest.TotalLoadTestTime = int64(loadTestEnd.Sub(loadTestStart).Seconds())
	loadTest.GetLoadTestMetrics()

	buffer, err := json.Marshal(&loadTest)
	if err != nil {
		fmt.Printf("error marshaling JSON: %v\n", err)
	}

	output := string(buffer)
	writeFile(output)
}
