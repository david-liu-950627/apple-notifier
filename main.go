package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

type Config struct {
	PartNumbers        []string `json:"partNumbers"`
	UserId             string   `json:"userId"`
	ChannelAccessToken string   `json:"channelAccessToken"`
}

type FulfillmentMessagesResponse struct {
	Head struct {
		Status string `json:"status"`
		Data   struct {
		} `json:"data"`
	} `json:"head"`
	Body struct {
		Content struct {
			PickupMessage struct {
				Stores []struct {
					StoreName         string `json:"storeName"`
					PartsAvailability map[string]struct {
						StorePickupProductTitle string `json:"storePickupProductTitle"`
						PickupDisplay           string `json:"pickupDisplay"`
					} `json:"partsAvailability"`
				} `json:"stores"`
			} `json:"pickupMessage"`
		} `json:"content"`
	} `json:"body"`
}

func main() {
	config, err := readConfig()
	if err != nil {
		log.Fatal(err)
	}
	lastPingTime := int64(0)

	for {
		fmt.Println("Start to check product...")
		messageLines := []string{}
		for idx, partNumber := range config.PartNumbers {
			fmt.Printf("%v. Checking %v ...\n", idx+1, partNumber)
			data, err := fetchProductInfo(partNumber)
			if err != nil {
				fmt.Println(err)
			}

			stores := data.Body.Content.PickupMessage.Stores
			for _, store := range stores {
				partsAvailability, ok := store.PartsAvailability[partNumber]
				if !ok {
					continue
				}

				isAvailable := partsAvailability.PickupDisplay == "available"

				if isAvailable {
					productTitle := partsAvailability.StorePickupProductTitle
					storeName := store.StoreName
					messageLines = append(messageLines, fmt.Sprintf("商品「%s」在「%s」可供訂購", productTitle, storeName))
				}
			}
		}
		timestampOfNow := time.Now().Unix()
		if timestampOfNow-lastPingTime >= 86400 {
			messageLines = append([]string{"Checking service is still alive!"}, messageLines...)
			lastPingTime = timestampOfNow
		}

		message := strings.Join(messageLines, "\n")
		pushMessageToLine(config.UserId, message, config.ChannelAccessToken)
		fmt.Print("Finish checking product.\n\n")

		time.Sleep(30 * time.Second)
	}
}

func fetchProductInfo(partNumber string) (data FulfillmentMessagesResponse, err error) {
	apiUrl, err := makeApiURL(partNumber)
	if err != nil {
		return
	}

	res, err := http.Get(apiUrl)
	if err != nil {
		return
	}

	defer res.Body.Close()
	jsonBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return
	}

	// unmarshal JSON to data
	json.Unmarshal(jsonBytes, &data)
	return
}

func makeApiURL(partNumber string) (url string, err error) {
	req, err := http.NewRequest("GET", "https://www.apple.com/tw/shop/fulfillment-messages", nil)
	if err != nil {
		return
	}

	q := req.URL.Query()
	q.Add("parts.0", partNumber)
	q.Add("location", "11061")
	q.Add("mt", "regular")
	q.Add("option.0", "")
	q.Add("_", fmt.Sprint(time.Now().UnixMilli()))
	req.URL.RawQuery = q.Encode()
	url = req.URL.String()
	return
}

func readConfig() (config Config, err error) {
	jsonBytes, err := ioutil.ReadFile("config.json")
	if err != nil {
		return
	}

	json.Unmarshal(jsonBytes, &config)
	return
}

func pushMessageToLine(userId string, message string, token string) {
	url := "https://api.line.me/v2/bot/message/push"

	values := map[string]interface{}{
		"to": userId,
		"messages": []map[string]interface{}{
			{"type": "text", "text": message},
		},
	}
	jsonValue, _ := json.Marshal(values)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonValue))
	if err != nil {
		fmt.Println(err)
		return
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %v", token))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	defer resp.Body.Close()
	if err != nil {
		fmt.Println(err)
		return
	}
}
