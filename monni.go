package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image/png"
	"image/jpeg"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/keep/v1"
	"google.golang.org/api/option"

	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/renderers"

	"gopkg.in/xmlpath.v2"
)

// Google Keep configuration
// =========================
var serviceAccountFilePath = "service-account.json"
const serviceAccountUser = "YOUR_EMAIL_HERE@DOMAIN.COM"
var notes = [...]string{
	"notes/YOUR_NOTE_ID_HERE",
	// Repeat for multiple notes.
}

// BOM configuration
// =================
// To look up a geohash, see https://api.weather.bom.gov.au/v1/locations?search=NNNN
// where NNNN is your postcode. Use the first 6 characters of the returned 7
// character geohash.
const weatherLocation = "YOUR_CITY_HERE"
// See https://www.bom.gov.au/catalogue/data-feeds.shtml
const bomProduct = "YOUR_BOM_PRODUCT_HERE"
const bomGeohash = "YOUR_GEOHASH_HERE"

// PTV configuration
// =================
// See https://www.vic.gov.au/public-transport-timetable-api
const ptvDevId = "YOUR_DEV_ID_HERE"
const ptvApiKey = "YOUR_API_KEY_HERE"
const ptvTrainStopId = 0
const ptvBusStopId = 0

const screenWidth = 600.0
const screenHeight = 800.0
const margin = screenWidth * 0.08

var fontFamily *canvas.FontFamily
var headerFace *canvas.FontFace
var textFace *canvas.FontFace
var textFaceBold *canvas.FontFace

func openKeep() (*keep.Service, error) {
	jsonCredentials, err := ioutil.ReadFile(serviceAccountFilePath)
	if err != nil {
		log.Fatalf("Failed to read service account info: %v.", err)
	}

	config, err := google.JWTConfigFromJSON(jsonCredentials, keep.KeepReadonlyScope)
	if err != nil {
		log.Fatalf("JWTConfigFromJSON: %v", err)
	}
	config.Subject = serviceAccountUser
	ts := config.TokenSource(context.Background())

	return keep.NewService(context.Background(), option.WithTokenSource(ts))
}

func fetchURLInsecure(url string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "text/plain")

	// wttr.in has something funky going on with certificates (or perhaps the
	// Kindle's root store is too old), so we skip verification.
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func drawText(ctx *canvas.Context, x float64, y float64, text *canvas.Text) float64 {
	h := text.Bounds().H
	ctx.DrawText(x, y, text)
	return h
}

func drawImage(ctx *canvas.Context, x float64, y float64, filename string) {
	f, err := os.Open(filename)
	if err != nil {
		log.Fatalf("Can't open image: %v", err)
	}
	img, err := png.Decode(f)
	if err != nil {
		log.Fatalf("Can't decode image: %v", err)
	}
	imgDPMM := 2.0
	w := float64(img.Bounds().Max.X) / imgDPMM
	h := float64(img.Bounds().Max.Y) / imgDPMM
	ctx.DrawImage(x-w/2, y-h/2, img, canvas.DPMM(imgDPMM))
}

func drawBgImage(ctx *canvas.Context, filename string) {
	f, err := os.Open(filename)
	if err != nil {
		log.Fatalf("Can't open image: %v", err)
	}
	img, err := jpeg.Decode(f)
	if err != nil {
		log.Fatalf("Can't decode image: %v", err)
	}
	rect := canvas.Rect{X: 0, Y: 0, W: ctx.Width(), H: ctx.Height()}
	ctx.FitImage(img, rect, canvas.ImageFill)
}

func getBatteryLevel() int {
	cmd := "powerd_test -s | grep -o -e 'Battery Level: [0-9]*' | cut -d ' ' -f3"
	out, err := exec.Command("sh", "-c", cmd).Output()
	if err != nil {
		log.Fatalf("Unable to read battery level: %v", err)
	}
	level, err := strconv.Atoi(strings.Trim(string(out), "\n"))
	if err != nil {
		log.Printf("Warning: Unable to read battery level: %v\n", err)
		return 100
	}
	return level
}

func drawNote(ctx *canvas.Context, note *keep.Note, x float64, y float64) float64 {
	ctx.SetFillColor(canvas.Black)

	origY := y
	y -= drawText(ctx, x, y,
		canvas.NewTextBox(headerFace, note.Title, screenWidth-margin-x, 0.0, canvas.Left, canvas.Top, 0.0, 0.0))
	y -= 8

	for _, item := range note.Body.List.ListItems {
		if item.Checked {
			continue
		}
		parts := strings.Split(item.Text.Text, "-")
		h := drawText(ctx, x, y,
			canvas.NewTextBox(textFace, parts[0], screenWidth-margin*2-x, 0.0, canvas.Left, canvas.Top, 0.0, 0.0))
		if len(parts) > 1 {
			x2 := x + 36
			drawText(ctx, x2, y,
				canvas.NewTextBox(textFace, "·", screenWidth-margin*2-x2, 0.0, canvas.Left, canvas.Top, 0.0, 0.0))
			x2 = x + 52
			h2 := drawText(ctx, x2, y,
				canvas.NewTextBox(textFace, strings.Trim(parts[1], " "), screenWidth-margin*2-x2, 0.0, canvas.Left, canvas.Top, 0.0, 0.0))
			if h2 > h {
				h = h2
			}
		}
		y -= h
	}
	return origY - y + 28
}

type ForecastRain struct {
	Chance int `json:"chance"`
}

type ForecastEntry struct {
	Rain        ForecastRain `json:"rain"`
	Temperature int          `json:"temp"`
	Time        string       `json:"time"`
	NextTime    string       `json:"next_forecast_period"`
}

type Forecast struct {
	Data []ForecastEntry `json:"data"`
}

func fetchForecast(geohash string) *Forecast {
	url := fmt.Sprintf("https://api.weather.bom.gov.au/v1/locations/%s/forecasts/3-hourly", geohash)
	client := http.Client{
		Timeout: time.Second * 20,
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Printf("Unable to build weather request: %v\n", err)
		return nil
	}
	res, err := client.Do(req)
	if err != nil {
		log.Printf("Unable to fetch weather data: %v\n", err)
		return nil
	}
	if res.Body != nil {
		defer res.Body.Close()
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Printf("Unable to fetch weather data: %v\n", err)
		return nil
	}
	forecast := Forecast{}
	err = json.Unmarshal(body, &forecast)
	if err != nil {
		log.Printf("Unable to parse weather data: %v\n", err)
		return nil
	}
	return &forecast
}

func drawWeather(ctx *canvas.Context, x float64, y float64) float64 {
	forecast := fetchForecast(bomGeohash)

	if forecast != nil {
		origY := y
		x := 20.0
		l := 0.0
		for _, entry := range forecast.Data {
			t, err := time.Parse(time.RFC3339, entry.Time)
			if err != nil {
				continue
			}
			l = drawText(ctx, x, y,
				canvas.NewTextBox(textFaceBold, fmt.Sprintf("%d:%02d", t.Local().Hour(), t.Local().Minute()), screenWidth-margin-x, 0.0, canvas.Left, canvas.Top, 0.0, 0.0))
			drawText(ctx, x, y-l,
				canvas.NewTextBox(textFace, fmt.Sprintf("%d°C", entry.Temperature), screenWidth-margin-x, 0.0, canvas.Left, canvas.Top, 0.0, 0.0))
			drawText(ctx, x, y-l*2,
				canvas.NewTextBox(textFace, fmt.Sprintf("%d%%", entry.Rain.Chance), screenWidth-margin-x, 0.0, canvas.Left, canvas.Top, 0.0, 0.0))
			x += screenWidth / 6
		}
		return origY - y + 28 + l*3
	}

	out, err := exec.Command("wget", "-qO", "-", "ftp://ftp.bom.gov.au/anon/gen/fwo/" + bomProduct + ".xml").Output()
	if err != nil {
		log.Println("Unable to read weather report: %v", err)
		return 0
	}
	path := xmlpath.MustCompile("//product/forecast/area[@description='" + weatherLocation + "']/forecast-period[1]/text")
	datePath := xmlpath.MustCompile("//product/forecast/area[@description='" + weatherLocation + "']/forecast-period[1]/@start-time-local")
	uvPath := xmlpath.MustCompile("//product/forecast/area[@description='" + weatherLocation + "']/forecast-period[1]/text[@type='uv_alert']")
	root, err := xmlpath.Parse(strings.NewReader(string(out)))
	if err != nil {
		log.Println("Unable to parse weather report: %v", err)
	}

	origY := y
	y -= drawText(ctx, x, y,
		canvas.NewTextBox(headerFace, "Weather", screenWidth-margin-x, 0.0, canvas.Left, canvas.Top, 0.0, 0.0))
	y -= 8

	weather := ""
	if value, ok := datePath.String(root); ok {
		t, err := time.Parse(time.RFC3339, value)
		if err == nil {
			if t.Hour() > 0 {
				weather += t.Format("Mon, Jan 2 · 15:04 · ")
			} else {
				weather += t.Format("Mon, Jan 2 · ")
			}
		}
	}

	if value, ok := path.String(root); ok {
		fmt.Println("BOM:", value)
		weather += value
	}

	if value, ok := uvPath.String(root); ok {
		fmt.Println("BOM UV:", value)
		if len(weather) > 0 {
			weather += " "
		}
		weather += value
	}

	weatherNow := ""
	wttr, err := fetchURLInsecure("https://wttr.in/" + weatherLocation + "?format=%C+%t+(feels+%f)+UV+%u+%w")
	if err == nil {
		fmt.Println("wttr.in:", wttr)
		parts := strings.SplitN(wttr, " ", 2)
		if len(parts) > 1 {
			weatherNow += parts[1]
		}
	} else {
		log.Printf("Warning: wttr.in failed: %v", err)
	}

	y -= drawText(ctx, x, y,
		canvas.NewTextBox(textFace, weather, screenWidth-margin-x, 0.0, canvas.Left, canvas.Top, 0.0, 0.0))

	y -= 12
	if y > 30 {
		y -= drawText(ctx, x, y,
			canvas.NewTextBox(textFace, weatherNow, screenWidth-margin-x, 0.0, canvas.Left, canvas.Top, 0.0, 0.0))
	}

	return origY - y + 28
}

type TransitRun struct {
	DestinationName string `json:"destination_name"`
	RunRef          string `json:"run_ref"`
}

type TransitDeparture struct {
	DisruptionIds      []int  `json:"disruption_ids"`
	RunRef             string `json:"run_ref"`
	ScheduledDeparture string `json:"scheduled_departure_utc"`
	EstimatedDeparture string `json:"estimated_departure_utc"`
}

type TransitDirection struct {
	DirectionId   int    `json:"direction_id"`
	DirectionName string `json:"direction_name"`
}

type Transit struct {
	Departures []TransitDeparture          `json:"departures"`
	Runs       map[string]TransitRun       `json:"runs"`
	Directions map[string]TransitDirection `json:"directions"`
}

func signPtvRequest(url string) string {
	devId := ptvDevId
	apiKey := ptvApiKey
	baseUrl := "https://timetableapi.ptv.vic.gov.au"

	url = url + "&devid=" + devId
	keyBytes := []byte(apiKey)
	h := hmac.New(sha1.New, keyBytes)
	h.Write([]byte(url))
	signatureBytes := h.Sum(nil)
	signature := hex.EncodeToString([]byte(signatureBytes))
	url += "&signature=" + strings.ToUpper(signature)
	return baseUrl + url
}

func fetchTransit(modeId int, stopId int) *Transit {
	url := fmt.Sprintf("/v3/departures/route_type/%d/stop/%d?include_cancelled=false&expand=%%5B%%22Run%22%2C%%22Direction%%22%%5D&", modeId, stopId)
	url = signPtvRequest(url)

	client := http.Client{
		Timeout: time.Second * 20,
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Printf("Unable to build transit request: %v\n", err)
		return nil
	}
	res, err := client.Do(req)
	if err != nil {
		log.Printf("Unable to fetch transit data: %v\n", err)
		return nil
	}
	if res.Body != nil {
		defer res.Body.Close()
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Printf("Unable to fetch transit data: %v\n", err)
		return nil
	}
	transit := Transit{}
	err = json.Unmarshal(body, &transit)
	if err != nil {
		log.Printf("Unable to parse transit data: %v\n", err)
		return nil
	}
	return &transit
}

func drawTransit(ctx *canvas.Context, x float64, y float64) float64 {
	trainMode := 0
	trainStopId := ptvTrainStopId
	busMode := 2
	busStopId := ptvBusStopId
	busTripMinutes := 10.0
	bus := fetchTransit(busMode, busStopId)
	train := fetchTransit(trainMode, trainStopId)
	if train == nil || bus == nil {
		return 0
	}

	origY := y
	x = 20.0
	for _, departure := range bus.Departures {
		depTime := departure.ScheduledDeparture
		if departure.EstimatedDeparture != "" {
			depTime = departure.EstimatedDeparture
		}
		t, err := time.Parse(time.RFC3339, depTime)
		if err != nil {
			continue
		}
		if t.Before(time.Now()) {
			continue
		}
		disruptions := ""
		if len(departure.DisruptionIds) > 0 {
			disruptions = "*"
		}
		l := drawText(ctx, x, y,
			canvas.NewTextBox(textFaceBold, fmt.Sprintf("%d:%02d%s", t.Local().Hour(), t.Local().Minute(), disruptions), screenWidth-margin-x, 0.0, canvas.Left, canvas.Top, 0.0, 0.0))

		savedY := y
		for _, trainDeparture := range train.Departures {
			trainDepTime := trainDeparture.ScheduledDeparture
			if trainDeparture.EstimatedDeparture != "" {
				trainDepTime = trainDeparture.EstimatedDeparture
			}
			_, ok := train.Runs[trainDeparture.RunRef]
			if !ok {
				continue
			}
			trainT, err := time.Parse(time.RFC3339, trainDepTime)
			if err != nil {
				continue
			}
			if trainT.Before(t) {
				continue
			}
			disruptions := ""
			if len(trainDeparture.DisruptionIds) > 0 {
				disruptions = "*"
			}
			relTime := trainT.Sub(t)
			if relTime.Minutes() < busTripMinutes {
				continue
			}
			if relTime.Minutes() > 40 {
				break
			}
			y -= l
			drawText(ctx, x, y,
				canvas.NewTextBox(textFace, fmt.Sprintf("+%.0f%s", relTime.Minutes(), disruptions), screenWidth-margin-x, 0.0, canvas.Left, canvas.Top, 0.0, 0.0))
		}
		y = savedY

		x += screenWidth / 6
	}
	return origY - y + 28
}

func main() {
	srv, err := openKeep()
	if err != nil {
		log.Fatalf("Failed to connect to Keep: %v.", err)
	}

	fontFamily = canvas.NewFontFamily("times")
	if err := fontFamily.LoadFontFile("fonts/static/Merriweather-Regular.ttf", canvas.FontRegular); err != nil {
		log.Fatalf("Failed to load font: %v.", err)
	}
	if err := fontFamily.LoadFontFile("fonts/static/Merriweather-Bold.ttf", canvas.FontBold); err != nil {
		log.Fatalf("Failed to load font: %v.", err)
	}
	headerFace = fontFamily.Face(2.5*30.0, canvas.Black, canvas.FontBold, canvas.FontNormal)
	textFace = fontFamily.Face(5*12.0, canvas.Black, canvas.FontRegular, canvas.FontNormal)
	textFaceBold = fontFamily.Face(5*12.0, canvas.Black, canvas.FontBold, canvas.FontNormal)

	c := canvas.New(screenWidth, screenHeight)
	ctx := canvas.NewContext(c)
	//ctx.SetFillColor(canvas.White)
	//ctx.DrawPath(0, 0, canvas.Rectangle(c.W, c.H))

	// Random background images; disabled for now.
	//files, err := os.ReadDir("images/bg")
	//if err == nil {
	//	rand.Seed(time.Now().UnixNano())
	//	i := rand.Intn(len(files))
	//	bgImage := "images/bg/" + files[i].Name()
	//	log.Printf("Background image: %v\n", bgImage)

	//	drawBgImage(ctx, bgImage)
	//	ctx.SetFillColor(color.RGBA{0xff, 0xff, 0xff, 0x88})
	//	ctx.DrawPath(0, 0, canvas.Rectangle(c.W, c.H))
	//}

	x := margin
	y := screenHeight - margin*0.5
	for _, noteId := range notes {
		note, err := srv.Notes.Get(noteId).Do()
		if err != nil {
			log.Fatalf("Unable to retrieve note data from Keep: %v", err)
		}
		fmt.Println("---", note.Title)
		y -= drawNote(ctx, note, x, y)
		for _, item := range note.Body.List.ListItems {
			checked := "[ ]"
			if item.Checked {
				checked = "[x]"
			}
			fmt.Println(checked, item.Text.Text)
			for _, childItem := range item.ChildListItems {
				checked = "[ ]"
				if item.Checked {
					checked = "[x]"
				}
				fmt.Println(" -", checked, childItem.Text)
			}
		}
	}

	y -= drawWeather(ctx, x, y)
	drawTransit(ctx, x, y)

	battery := getBatteryLevel()
	batteryIcon := ""
	if battery < 1*100/8 {
		batteryIcon = "images/battery0.png"
	} else if battery < 2*100/8 {
		batteryIcon = "images/battery1.png"
	} else if battery < 3*100/8 {
		batteryIcon = "images/battery2.png"
	} else if battery < 4*100/8 {
		batteryIcon = "images/battery3.png"
	} else if battery < 5*100/8 {
		batteryIcon = "images/battery4.png"
	} else if battery < 6*100/8 {
		batteryIcon = "images/battery5.png"
	} else if battery < 7*100/8 {
		batteryIcon = "images/battery6.png"
	} else {
		batteryIcon = "images/battery7.png"
	}
	y = screenHeight - margin*0.85
	drawImage(ctx, screenWidth-margin, y, batteryIcon)

	y -= 8 + 16
	currentTime := time.Now()
	y -= drawText(ctx, x, y,
		canvas.NewTextBox(textFace, currentTime.Format("Jan 02"), screenWidth-margin*0.85-x, 0.0, canvas.Right, canvas.Top, 0.0, 0.0))
	y -= drawText(ctx, x, y,
		canvas.NewTextBox(textFace, currentTime.Format("15:04"), screenWidth-margin*0.85-x, 0.0, canvas.Right, canvas.Top, 0.0, 0.0))

	log.Printf("Saving image to out.png")
	renderers.Write("out.png", c, canvas.DPI(25.38))
	log.Printf("Finished")
}
