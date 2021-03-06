package main

import (
	"encoding/json"
	"fmt"
	"github.com/mattn/go-colorable"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type configuration struct {
	APIKey   string
	City     string
	Imperial bool
}

type cond struct {
	ChanceOfRain   string  `json:"chanceofrain"`
	FeelsLikeC     int     `json:",string"`
	PrecipMM       float32 `json:"precipMM,string"`
	TempC          int     `json:"tempC,string"`
	TempC2         int     `json:"temp_C,string"`
	Time           int     `json:"time,string"`
	VisibleDistKM  int     `json:"visibility,string"`
	WeatherCode    int     `json:"weatherCode,string"`
	WeatherDesc    []struct{ Value string }
	WindGustKmph   int `json:",string"`
	Winddir16Point string
	WindspeedKmph  int `json:"windspeedKmph,string"`
}

type astro struct {
	Moonrise string
	Moonset  string
	Sunrise  string
	Sunset   string
}

type weather struct {
	Astronomy []astro
	Date      string
	Hourly    []cond
	MaxtempC  int `json:"maxtempC,string"`
	MintempC  int `json:"mintempC,string"`
}

type loc struct {
	Query string `json:"query"`
	Type  string `json:"type"`
}

type resp struct {
	Data struct {
		Cur     []cond                 `json:"current_condition"`
		Err     []struct{ Msg string } `json:"error"`
		Req     []loc                  `json:"request"`
		Weather []weather              `json:"weather"`
	} `json:"data"`
}

var (
	config     configuration
	configpath string
	params     []string
	windDir    = map[string]string{
		"N":   "\033[1m↓\033[0m",
		"NNE": "\033[1m↓\033[0m",
		"NE":  "\033[1m↙\033[0m",
		"ENE": "\033[1m↙\033[0m",
		"E":   "\033[1m←\033[0m",
		"ESE": "\033[1m←\033[0m",
		"SE":  "\033[1m↖\033[0m",
		"SSE": "\033[1m↖\033[0m",
		"S":   "\033[1m↑\033[0m",
		"SSW": "\033[1m↑\033[0m",
		"SW":  "\033[1m↗\033[0m",
		"WSW": "\033[1m↗\033[0m",
		"W":   "\033[1m→\033[0m",
		"WNW": "\033[1m→\033[0m",
		"NW":  "\033[1m↘\033[0m",
		"NNW": "\033[1m↘\033[0m",
	}
	unitRain = map[bool]string{
		false: "mm",
		true:  "in",
	}
	unitTemp = map[bool]string{
		false: "C",
		true:  "F",
	}
	unitVis = map[bool]string{
		false: "km",
		true:  "mi",
	}
	unitWind = map[bool]string{
		false: "km/h",
		true:  "mph",
	}
	slotTimes = [slotcount]int{9 * 60, 12 * 60, 18 * 60, 22 * 60}
	codes     = map[int][]string{
		113: iconSunny,
		116: iconPartlyCloudy,
		119: iconCloudy,
		122: iconVeryCloudy,
		143: iconFog,
		176: iconLightShowers,
		179: iconLightSleetShowers,
		182: iconLightSleet,
		185: iconLightSleet,
		200: iconThunderyShowers,
		227: iconLightSnow,
		230: iconHeavySnow,
		248: iconFog,
		260: iconFog,
		263: iconLightShowers,
		266: iconLightRain,
		281: iconLightSleet,
		284: iconLightSleet,
		293: iconLightRain,
		296: iconLightRain,
		299: iconHeavyShowers,
		302: iconHeavyRain,
		305: iconHeavyShowers,
		308: iconHeavyRain,
		311: iconLightSleet,
		314: iconLightSleet,
		317: iconLightSleet,
		320: iconLightSnow,
		323: iconLightSnowShowers,
		326: iconLightSnowShowers,
		329: iconHeavySnow,
		332: iconHeavySnow,
		335: iconHeavySnowShowers,
		338: iconHeavySnow,
		350: iconLightSleet,
		353: iconLightShowers,
		356: iconHeavyShowers,
		359: iconHeavyRain,
		362: iconLightSleetShowers,
		365: iconLightSleetShowers,
		368: iconLightSnowShowers,
		371: iconHeavySnowShowers,
		374: iconLightSleetShowers,
		377: iconLightSleet,
		386: iconThunderyShowers,
		389: iconThunderyHeavyRain,
		392: iconThunderySnowShowers,
		395: iconHeavySnowShowers, // ThunderyHeavySnow
	}

	iconUnknown = []string{
		"    .-.      ",
		"     __)     ",
		"    (        ",
		"     `-’     ",
		"      •      "}
	iconSunny = []string{
		"\033[38;5;226m    \\   /    \033[0m",
		"\033[38;5;226m     .-.     \033[0m",
		"\033[38;5;226m  ― (   ) ―  \033[0m",
		"\033[38;5;226m     `-’     \033[0m",
		"\033[38;5;226m    /   \\    \033[0m"}
	iconPartlyCloudy = []string{
		"\033[38;5;226m   \\  /\033[0m      ",
		"\033[38;5;226m _ /\"\"\033[38;5;250m.-.    \033[0m",
		"\033[38;5;226m   \\_\033[38;5;250m(   ).  \033[0m",
		"\033[38;5;226m   /\033[38;5;250m(___(__) \033[0m",
		"             "}
	iconCloudy = []string{
		"             ",
		"\033[38;5;250m     .--.    \033[0m",
		"\033[38;5;250m  .-(    ).  \033[0m",
		"\033[38;5;250m (___.__)__) \033[0m",
		"             "}
	iconVeryCloudy = []string{
		"             ",
		"\033[38;5;240;1m     .--.    \033[0m",
		"\033[38;5;240;1m  .-(    ).  \033[0m",
		"\033[38;5;240;1m (___.__)__) \033[0m",
		"             "}
	iconLightShowers = []string{
		"\033[38;5;226m _`/\"\"\033[38;5;250m.-.    \033[0m",
		"\033[38;5;226m  ,\\_\033[38;5;250m(   ).  \033[0m",
		"\033[38;5;226m   /\033[38;5;250m(___(__) \033[0m",
		"\033[38;5;111m     ‘ ‘ ‘ ‘ \033[0m",
		"\033[38;5;111m    ‘ ‘ ‘ ‘  \033[0m"}
	iconHeavyShowers = []string{
		"\033[38;5;226m _`/\"\"\033[38;5;240;1m.-.    \033[0m",
		"\033[38;5;226m  ,\\_\033[38;5;240;1m(   ).  \033[0m",
		"\033[38;5;226m   /\033[38;5;240;1m(___(__) \033[0m",
		"\033[38;5;21;1m   ‚‘‚‘‚‘‚‘  \033[0m",
		"\033[38;5;21;1m   ‚’‚’‚’‚’  \033[0m"}
	iconLightSnowShowers = []string{
		"\033[38;5;226m _`/\"\"\033[38;5;250m.-.    \033[0m",
		"\033[38;5;226m  ,\\_\033[38;5;250m(   ).  \033[0m",
		"\033[38;5;226m   /\033[38;5;250m(___(__) \033[0m",
		"\033[38;5;255m     *  *  * \033[0m",
		"\033[38;5;255m    *  *  *  \033[0m"}
	iconHeavySnowShowers = []string{
		"\033[38;5;226m _`/\"\"\033[38;5;240;1m.-.    \033[0m",
		"\033[38;5;226m  ,\\_\033[38;5;240;1m(   ).  \033[0m",
		"\033[38;5;226m   /\033[38;5;240;1m(___(__) \033[0m",
		"\033[38;5;255;1m    * * * *  \033[0m",
		"\033[38;5;255;1m   * * * *   \033[0m"}
	iconLightSleetShowers = []string{
		"\033[38;5;226m _`/\"\"\033[38;5;250m.-.    \033[0m",
		"\033[38;5;226m  ,\\_\033[38;5;250m(   ).  \033[0m",
		"\033[38;5;226m   /\033[38;5;250m(___(__) \033[0m",
		"\033[38;5;111m     ‘ \033[38;5;255m*\033[38;5;111m ‘ \033[38;5;255m* \033[0m",
		"\033[38;5;255m    *\033[38;5;111m ‘ \033[38;5;255m*\033[38;5;111m ‘  \033[0m"}
	iconThunderyShowers = []string{
		"\033[38;5;226m _`/\"\"\033[38;5;250m.-.    \033[0m",
		"\033[38;5;226m  ,\\_\033[38;5;250m(   ).  \033[0m",
		"\033[38;5;226m   /\033[38;5;250m(___(__) \033[0m",
		"\033[38;5;228;5m    ⚡\033[38;5;111;25m‘ ‘\033[38;5;228;5m⚡\033[38;5;111;25m‘ ‘ \033[0m",
		"\033[38;5;111m    ‘ ‘ ‘ ‘  \033[0m"}
	iconThunderyHeavyRain = []string{
		"\033[38;5;240;1m     .-.     \033[0m",
		"\033[38;5;240;1m    (   ).   \033[0m",
		"\033[38;5;240;1m   (___(__)  \033[0m",
		"\033[38;5;21;1m  ‚‘\033[38;5;228;5m⚡\033[38;5;21;25m‘‚\033[38;5;228;5m⚡\033[38;5;21;25m‚‘   \033[0m",
		"\033[38;5;21;1m  ‚’‚’\033[38;5;228;5m⚡\033[38;5;21;25m’‚’   \033[0m"}
	iconThunderySnowShowers = []string{
		"\033[38;5;226m _`/\"\"\033[38;5;250m.-.    \033[0m",
		"\033[38;5;226m  ,\\_\033[38;5;250m(   ).  \033[0m",
		"\033[38;5;226m   /\033[38;5;250m(___(__) \033[0m",
		"\033[38;5;255m     *\033[38;5;228;5m⚡\033[38;5;255;25m *\033[38;5;228;5m⚡\033[38;5;255;25m * \033[0m",
		"\033[38;5;255m    *  *  *  \033[0m"}
	iconLightRain = []string{
		"\033[38;5;250m     .-.     \033[0m",
		"\033[38;5;250m    (   ).   \033[0m",
		"\033[38;5;250m   (___(__)  \033[0m",
		"\033[38;5;111m    ‘ ‘ ‘ ‘  \033[0m",
		"\033[38;5;111m   ‘ ‘ ‘ ‘   \033[0m"}
	iconHeavyRain = []string{
		"\033[38;5;240;1m     .-.     \033[0m",
		"\033[38;5;240;1m    (   ).   \033[0m",
		"\033[38;5;240;1m   (___(__)  \033[0m",
		"\033[38;5;21;1m  ‚‘‚‘‚‘‚‘   \033[0m",
		"\033[38;5;21;1m  ‚’‚’‚’‚’   \033[0m"}
	iconLightSnow = []string{
		"\033[38;5;250m     .-.     \033[0m",
		"\033[38;5;250m    (   ).   \033[0m",
		"\033[38;5;250m   (___(__)  \033[0m",
		"\033[38;5;255m    *  *  *  \033[0m",
		"\033[38;5;255m   *  *  *   \033[0m"}
	iconHeavySnow = []string{
		"\033[38;5;240;1m     .-.     \033[0m",
		"\033[38;5;240;1m    (   ).   \033[0m",
		"\033[38;5;240;1m   (___(__)  \033[0m",
		"\033[38;5;255;1m   * * * *   \033[0m",
		"\033[38;5;255;1m  * * * *    \033[0m"}
	iconLightSleet = []string{
		"\033[38;5;250m     .-.     \033[0m",
		"\033[38;5;250m    (   ).   \033[0m",
		"\033[38;5;250m   (___(__)  \033[0m",
		"\033[38;5;111m    ‘ \033[38;5;255m*\033[38;5;111m ‘ \033[38;5;255m*  \033[0m",
		"\033[38;5;255m   *\033[38;5;111m ‘ \033[38;5;255m*\033[38;5;111m ‘   \033[0m"}
	iconFog = []string{
		"             ",
		"\033[38;5;251m _ - _ - _ - \033[0m",
		"\033[38;5;251m  _ - _ - _  \033[0m",
		"\033[38;5;251m _ - _ - _ - \033[0m",
		"             "}
)

const (
	uri       = "https://api.worldweatheronline.com/free/v2/weather.ashx?"
	slotcount = 4
)

func configload() error {
	b, err := ioutil.ReadFile(configpath)
	if err == nil {
		return json.Unmarshal(b, &config)
	}
	return err
}

func configsave() error {
	j, err := json.MarshalIndent(config, "", "\t")
	if err == nil {
		return ioutil.WriteFile(configpath, j, 0600)
	}
	return err
}

func formatTemp(c cond) string {
	color := func(temp int) string {
		var col = 21
		switch temp {
		case -15, -14, -13:
			col = 27
		case -12, -11, -10:
			col = 33
		case -9, -8, -7:
			col = 39
		case -6, -5, -4:
			col = 45
		case -3, -2, -1:
			col = 51
		case 0, 1:
			col = 50
		case 2, 3:
			col = 49
		case 4, 5:
			col = 48
		case 6, 7:
			col = 47
		case 8, 9:
			col = 46
		case 10, 11, 12:
			col = 82
		case 13, 14, 15:
			col = 118
		case 16, 17, 18:
			col = 154
		case 19, 20, 21:
			col = 190
		case 22, 23, 24:
			col = 226
		case 25, 26, 27:
			col = 220
		case 28, 29, 30:
			col = 214
		case 31, 32, 33:
			col = 208
		case 34, 35, 36:
			col = 202
		default:
			if temp > 0 {
				col = 196
			}
		}
		tempUnit := float32(temp)
		if config.Imperial {
			tempUnit = float32(temp)*1.8 + 32.0
		}
		return fmt.Sprintf("\033[38;5;%03dm%d\033[0m", col, int32(tempUnit))
	}
	t := c.TempC
	if t == 0 {
		t = c.TempC2
	}
	if c.FeelsLikeC < t {
		return fmt.Sprintf("%s – %s °%s         ", color(c.FeelsLikeC), color(t), unitTemp[config.Imperial])[:48]
	} else if c.FeelsLikeC > t {
		return fmt.Sprintf("%s – %s °%s         ", color(t), color(c.FeelsLikeC), unitTemp[config.Imperial])[:48]
	}
	return fmt.Sprintf("%s °%s            ", color(c.FeelsLikeC), unitTemp[config.Imperial])[:31]
}

func formatWind(c cond) string {
	color := func(spd int) string {
		var col = 46
		switch spd {
		case 1, 2, 3:
			col = 82
		case 4, 5, 6:
			col = 118
		case 7, 8, 9:
			col = 154
		case 10, 11, 12:
			col = 190
		case 13, 14, 15:
			col = 226
		case 16, 17, 18, 19:
			col = 220
		case 20, 21, 22, 23:
			col = 214
		case 24, 25, 26, 27:
			col = 208
		case 28, 29, 30, 31:
			col = 202
		default:
			if spd > 0 {
				col = 196
			}
		}
		spdUnit := float32(spd)
		if config.Imperial {
			spdUnit = float32(spd) / 1.609
		}
		return fmt.Sprintf("\033[38;5;%03dm%d\033[0m", col, int32(spdUnit))
	}
	if c.WindGustKmph > c.WindspeedKmph {
		return fmt.Sprintf("%s %s – %s %s     ", windDir[c.Winddir16Point], color(c.WindspeedKmph), color(c.WindGustKmph), unitWind[config.Imperial])[:57]
	}
	return fmt.Sprintf("%s %s %s        ", windDir[c.Winddir16Point], color(c.WindspeedKmph), unitWind[config.Imperial])[:40]
}

func formatVisibility(c cond) string {
	distUnit := float32(c.VisibleDistKM)
	if config.Imperial {
		distUnit = float32(c.VisibleDistKM) * 0.621
	}
	return fmt.Sprintf("%d %s            ", int32(distUnit), unitVis[config.Imperial])[:15]
}

func formatRain(c cond) string {
	rainUnit := float32(c.PrecipMM)
	if config.Imperial {
		rainUnit = float32(c.PrecipMM) * 0.039
	}
	if c.ChanceOfRain != "" {
		return fmt.Sprintf("%.1f %s | %s%%        ", rainUnit, unitRain[config.Imperial], c.ChanceOfRain)[:15]
	}
	return fmt.Sprintf("%.1f %s            ", rainUnit, unitRain[config.Imperial])[:15]
}

func formatCond(cur []string, c cond, current bool) (ret []string) {
	var icon []string
	if i, ok := codes[c.WeatherCode]; !ok {
		icon = iconUnknown
	} else {
		icon = i
	}
	desc := fmt.Sprintf("%-15.15v", c.WeatherDesc[0].Value)
	if current {
		desc = c.WeatherDesc[0].Value
	} else if lastRune, size := utf8.DecodeLastRuneInString(desc); lastRune != ' ' {
		desc = desc[:len(desc)-size] + "…"
	}
	ret = append(ret, fmt.Sprintf("%v %v %v", cur[0], icon[0], desc))
	ret = append(ret, fmt.Sprintf("%v %v %v", cur[1], icon[1], formatTemp(c)))
	ret = append(ret, fmt.Sprintf("%v %v %v", cur[2], icon[2], formatWind(c)))
	ret = append(ret, fmt.Sprintf("%v %v %v", cur[3], icon[3], formatVisibility(c)))
	ret = append(ret, fmt.Sprintf("%v %v %v", cur[4], icon[4], formatRain(c)))
	return
}

func printDay(w weather) (ret []string) {
	hourly := w.Hourly
	ret = make([]string, 5)
	for i := range ret {
		ret[i] = "│"
	}

	// find hourly data which fits the desired times of day best
	var slots [slotcount]cond
	for _, h := range hourly {
		c := int(math.Mod(float64(h.Time), 100)) + 60*(h.Time/100)
		for i, s := range slots {
			if math.Abs(float64(c-slotTimes[i])) < math.Abs(float64(s.Time-slotTimes[i])) {
				h.Time = c
				slots[i] = h
			}
		}
	}

	for _, s := range slots {
		ret = formatCond(ret, s, false)
		for i := range ret {
			ret[i] = ret[i] + "│"
		}
	}

	d, _ := time.Parse("2006-01-02", w.Date)
	dateFmt := "┤ " + d.Format("Mon 02. Jan") + " ├"
	ret = append([]string{
		"                                                       ┌─────────────┐                                                       ",
		"┌──────────────────────────────┬───────────────────────" + dateFmt + "───────────────────────┬──────────────────────────────┐",
		"│           Morning            │             Noon      └──────┬──────┘    Evening            │            Night             │",
		"├──────────────────────────────┼──────────────────────────────┼──────────────────────────────┼──────────────────────────────┤"},
		ret...)
	return append(ret,
		"└──────────────────────────────┴──────────────────────────────┴──────────────────────────────┴──────────────────────────────┘")
	return
}

func init() {
	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	configpath = path.Join(usr.HomeDir, ".wegorc")
	config.APIKey = ""
	config.City = "New York"
	config.Imperial = false
	err = configload()
	if _, ok := err.(*os.PathError); ok {
		log.Printf("No config file found. Creating %s ...", configpath)
		if err := configsave(); err != nil {
			log.Fatal(err)
		}
	} else if err != nil {
		log.Fatalf("could not parse %v: %v", configpath, err)
	}
}

func main() {
	var numdays = 3

	if len(config.APIKey) == 0 {
		log.Fatal("No API key specified. Setup instructions are in the README.")
	}
	params = append(params, "key="+config.APIKey)

	for _, arg := range os.Args[1:] {
		if v, err := strconv.Atoi(arg); err == nil && len(arg) == 1 {
			numdays = v
		} else {
			config.City = arg
		}
	}

	if len(config.City) > 0 {
		params = append(params, "q="+url.QueryEscape(config.City))
	}
	params = append(params, "format=json")
	params = append(params, "num_of_days="+strconv.Itoa(numdays))
	params = append(params, "tp=3")
	params = append(params, "lang=de")

	// fmt.Fprintln(os.Stderr, params)

	res, err := http.Get(uri + strings.Join(params, "&"))
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Fatal(err)
	}

	// fmt.Println(string(body))

	var r resp
	if err = json.Unmarshal(body, &r); err != nil {
		log.Println(err)
	}

	if r.Data.Req == nil || len(r.Data.Req) < 1 {
		if r.Data.Err != nil && len(r.Data.Err) >= 1 {
			log.Fatal(r.Data.Err[0].Msg)
		}
		log.Fatal("Malformed response.")
	}
	fmt.Printf("Weather for %s: %s\n\n", r.Data.Req[0].Type, r.Data.Req[0].Query)
	stdout := colorable.NewColorableStdout()

	if r.Data.Cur == nil || len(r.Data.Cur) < 1 {
		log.Fatal("No weather data available.")
	}
	out := formatCond(make([]string, 5), r.Data.Cur[0], true)
	for _, val := range out {
		fmt.Fprintln(stdout, val)
	}

	if numdays == 0 {
		return
	}
	if r.Data.Weather == nil {
		log.Fatal("No detailed weather forecast available.")
	}
	for _, d := range r.Data.Weather {
		for _, val := range printDay(d) {
			fmt.Fprintln(stdout, val)
		}
	}
}
