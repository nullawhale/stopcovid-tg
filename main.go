package main

import (
	"encoding/json"
	"fmt"
	"github.com/dustin/go-humanize"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/tkanos/gonfig"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

type Conf struct {
	Token        string
	MapDataUrl   string
	CovidInfoUrl string
}

type Region struct {
	Id    string `json:"id"`
	Title string `json:"title"`
}

type MapData struct {
	Items []Items `json:"Items"`
}

type Items struct {
	Confirmed    int64  `json:"Confirmed"`
	Deaths       int64  `json:"Deaths"`
	IsoCode      string `json:"IsoCode"`
	Lat          string `json:"Lat"`
	Lng          string `json:"Lng"`
	LocationName string `json:"LocationName"`
	New          string `json:"New"`
	Observations string `json:"Observations"`
	Recovered    int64  `json:"Recovered"`
}

type CovidInfo struct {
	Date   string `json:"date"`
	Sick   int64  `json:"sick,string"`
	Healed int64  `json:"healed,string"`
	Died   int64  `json:"died,string"`
}

type Currency struct {
	Ccy     string `json:"ccy"`
	BaseCcy string `json:"base_ccy"`
	Buy     string `json:"buy"`
	Sale    string `json:"sale"`
}

type CurrenciesCollection []Currency

var conf Conf

func main() {
	err := gonfig.GetConf("top.secret.json", &conf)
	if err != nil {
		log.Fatal(err)
	}

	bot, err := tgbotapi.NewBotAPI(conf.Token)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		log.Printf("User [%s] %s", update.Message.From.UserName, update.Message.Text)

		var msg tgbotapi.MessageConfig
		if update.Message.IsCommand() {
			msg = tgbotapi.NewMessage(update.Message.Chat.ID, genReply(update.Message.Command()))
		} else {
			msg = tgbotapi.NewMessage(update.Message.Chat.ID, genReply(update.Message.Text))
		}
		msg.ReplyToMessageID = update.Message.MessageID
		msg.ParseMode = "markdown"

		_, err := bot.Send(msg)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func genReply(message string) string {
	var reply string

	help := fmt.Sprintf("Доступные регионы:\n" +
		"\t/mos - Московская область\n" +
		"\t/mow - Москва\n" +
		"\t/yar - Ярославская область\n" +
		"\t/spb - Санкт-Петербург\n" +
		"\t/lenobl - Ленинградская область\n" +
		"\t/kda - Краснодарский край\n" +
		"\t/rusyar - Россия и Ярославская область\n" +
		"\t/rus - По всей России\n" +
		"\t/troll - Постебать Айфонодрочеров")

	switch message {
	case "mos":
		reply = getCovidInfoString("RU-MOS")
	case "mow":
		reply = getCovidInfoString("RU-MOW")
	case "yar":
		reply = getCovidInfoString("RU-YAR")
	case "kda":
		reply = getCovidInfoString("RU-KDA")
	case "spb":
		reply = getCovidInfoString("RU-SPE")
	case "lenobl":
		reply = getCovidInfoString("RU-LEN")
	case "rus":
		reply = allRussia()
	case "rusyar":
		reply = fmt.Sprintf("%s\n%s", allRussia(), getCovidInfoString("RU-YAR"))
	case "cur":
		reply = getCurrencyReply()
	case "help":
		reply = help
	case "start":
		reply = help
	case "troll":
		reply = "iPhone - говно, Android - сила. "
	}

	return reply
}

func allRussia() string {
	var allConfirmed int64
	var allDeaths int64
	var allRecovered int64
	regions := getMapData().Items
	for _, value := range regions {
		allConfirmed += value.Confirmed
		allDeaths += value.Deaths
		allRecovered += value.Recovered
	}
	return fmt.Sprintf("Общее по России: \n \tУмерло: %s\n \tВыявлено: %s\n \tВыздоровело: %s \n",
		humanize.Comma(allDeaths),
		humanize.Comma(allConfirmed),
		humanize.Comma(allRecovered),
	)
}

func getRegions() []Region {
	jsonFile, err := os.Open("regions.json")
	bodyBytes, _ := ioutil.ReadAll(jsonFile)
	if err != nil {
		fmt.Println(err)
	}
	var regions []Region
	json.Unmarshal(bodyBytes, &regions)
	defer jsonFile.Close()

	return regions
}

func getCovidInfoString(region string) string {
	var result string
	var title string
	regions := getRegions()
	for _, value := range regions {
		if value.Id == region {
			title = value.Title
		}
	}

	covidInfo := getCovidInfo(region)

	var todayInfo, yesterdayInfo CovidInfo
	todayInfo = covidInfo[0]
	yesterdayInfo = covidInfo[1]

	if todayInfo.Date != "" {
		result = fmt.Sprintf("%s\n \tДата: %s\n \tУмерло: %s (+%s)\n \tВыявлено: %s (+%s)\n \tВыздоровело: %s (+%s)\n",
			title,
			todayInfo.Date,
			humanize.Comma(todayInfo.Died), humanize.Comma(todayInfo.Died-yesterdayInfo.Died),
			humanize.Comma(todayInfo.Sick), humanize.Comma(todayInfo.Sick-yesterdayInfo.Sick),
			humanize.Comma(todayInfo.Healed), humanize.Comma(todayInfo.Healed-yesterdayInfo.Healed))
	} else if yesterdayInfo.Date != "" {
		result = fmt.Sprintf("%s\n \tДата: %s\n \tУмерло: %s\n \tВыявлено: %s\n \tВыздоровело: %s\n",
			title,
			yesterdayInfo.Date,
			humanize.Comma(yesterdayInfo.Died),
			humanize.Comma(yesterdayInfo.Sick),
			humanize.Comma(yesterdayInfo.Healed))
	}
	return result
}

func getCovidInfo(region string) []CovidInfo {
	url := conf.CovidInfoUrl
	resp, err := http.Get(fmt.Sprintf("%s=%s", url, region))
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()
	bodyBytes, _ := ioutil.ReadAll(resp.Body)

	var covidInfo []CovidInfo
	json.Unmarshal(bodyBytes, &covidInfo)

	return covidInfo
}

func getMapData() MapData {
	url := conf.MapDataUrl
	resp, err := http.Get(fmt.Sprintf("%s", url))
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()
	bodyBytes, _ := ioutil.ReadAll(resp.Body)

	var mapData MapData
	json.Unmarshal(bodyBytes, &mapData)

	return mapData
}

func getCurrencyReply() string {
	curCollection := getCurrenciesCollection()
	return exchangeRatesToString(curCollection)
}

func getCurrenciesCollection() *CurrenciesCollection {

	url := "https://api.privatbank.ua/p24api/pubinfo?json&exchange&coursid=5"

	resp, err := http.Get(url)

	if err != nil {
		panic(err.Error())
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err.Error())
	}

	currencyData, err := parseCurrencies([]byte(body))

	return currencyData
}

func parseCurrencies(body []byte) (*CurrenciesCollection, error) {
	var s = new(CurrenciesCollection)
	err := json.Unmarshal(body, &s)
	if err != nil {
		fmt.Println("whoops:", err)
	}
	return s, err
}

func exchangeRatesToString(currenciesCollection CurrenciesCollection) string {
	var message string

	for i := 0; i < len(currenciesCollection); i++ {
		currency := currenciesCollection[i]
		fmt.Println(currency.Sale)
		message += currency.Ccy + "\n"
		message += "- sale " + currency.Sale + " " + currency.BaseCcy + "\n"
		message += "- buy " + currency.Buy + " " + currency.BaseCcy + "\n"
	}

	return message
}
