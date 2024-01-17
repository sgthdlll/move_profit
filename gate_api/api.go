package gate_api

import (
	"context"
	"fmt"
	"github.com/antihax/optional"
	gateapi "github.com/gateio/gateapi-go/v6"
	"net/http"
	"sync"
	"time"
)

var client *gateapi.APIClient

var gateMarketInfoMap sync.Map

func InitGateClient() {
	client = getGateApiClient()
	gateMarketInfoList, _ := GetGateMarketInfo()
	for _, contract := range gateMarketInfoList {
		gateMarketInfoMap.Store(contract.Name, contract)
	}
}

func GetMarketInfo(market string) (gateapi.Contract, bool) {
	val, ok := gateMarketInfoMap.Load(market)
	if !ok {
		return gateapi.Contract{}, false
	}
	return val.(gateapi.Contract), true
}

func GetGateMarketInfo() ([]gateapi.Contract, error) {
	ctx := context.WithValue(context.Background(), gateapi.ContextGateAPIV4, gateapi.GateAPIV4{
		Key:    "f6f3cc50911e00ae4ff6a5dbb1913a5e",
		Secret: "8ea94f2850ad01d1da565e5c2ef6369e9667cb18209f676120857d5bc8f42c34",
	})
	contractList, _, err := client.FuturesApi.ListFuturesContracts(ctx, "usdt")
	if err != nil {
		return nil, err
	}
	return contractList, nil
}

func getGateApiClient() *gateapi.APIClient {
	cfg := gateapi.NewConfiguration()
	cfg.HTTPClient = &http.Client{Timeout: 60 * time.Second}
	return gateapi.NewAPIClient(cfg)
}

func PlaceExchagneOrder(market string, size int) (gateapi.FuturesOrder, error) {
	reqOrder := gateapi.FuturesOrder{
		Contract: market,
		Price:    "0",
		Size:     int64(size),
		Tif:      "ioc",
	}
	ctx := context.WithValue(context.Background(), gateapi.ContextGateAPIV4, gateapi.GateAPIV4{
		Key:    "f6f3cc50911e00ae4ff6a5dbb1913a5e",
		Secret: "8ea94f2850ad01d1da565e5c2ef6369e9667cb18209f676120857d5bc8f42c34",
	})

	orderResponse, _, err := client.FuturesApi.CreateFuturesOrder(ctx, "usdt", reqOrder)
	if err != nil {
		return gateapi.FuturesOrder{}, err
	}
	return orderResponse, nil
}
func SwitchPositionLeverage(market string, leverage int) error {
	ctx := context.WithValue(context.Background(), gateapi.ContextGateAPIV4, gateapi.GateAPIV4{
		Key:    "f6f3cc50911e00ae4ff6a5dbb1913a5e",
		Secret: "8ea94f2850ad01d1da565e5c2ef6369e9667cb18209f676120857d5bc8f42c34",
	})
	_, _, err := client.FuturesApi.UpdatePositionLeverage(ctx, "usdt", market, "0", &gateapi.UpdatePositionLeverageOpts{CrossLeverageLimit: optional.NewString(fmt.Sprintf("%d", leverage))})
	if err != nil {
		return err
	}
	return nil
}

func SwitchPositionMode() error {
	ctx := context.WithValue(context.Background(), gateapi.ContextGateAPIV4, gateapi.GateAPIV4{
		Key:    "f6f3cc50911e00ae4ff6a5dbb1913a5e",
		Secret: "8ea94f2850ad01d1da565e5c2ef6369e9667cb18209f676120857d5bc8f42c34",
	})
	_, _, err := client.FuturesApi.SetDualMode(ctx, "usdt", false)
	if err != nil {
		return err
	}
	return nil
}
