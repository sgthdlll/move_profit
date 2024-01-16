package ws

import (
	"encoding/json"
	"io"
	"net/http"
	"quant/utils"
)

type MarketInfo struct {
	Name string `json:"name"`
}

func getGateMarketInfo() []string {
	api := "https://api.gateio.ws/api/v4/futures/usdt/contracts"

	req, err := http.NewRequest(http.MethodGet, api, nil)
	if err != nil {
		return []string{}
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return []string{}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return []string{}
	}

	if !utils.InArray(resp.StatusCode, []int{http.StatusOK, http.StatusCreated, http.StatusNoContent}) {
		return []string{}
	}
	var marketInfoList = make([]*MarketInfo, 0)
	err = json.Unmarshal(body, &marketInfoList)
	if err != nil {
		return []string{}
	}
	marketList := make([]string, 0, len(marketInfoList))
	for _, info := range marketInfoList {
		marketList = append(marketList, info.Name)
	}
	return marketList
}
