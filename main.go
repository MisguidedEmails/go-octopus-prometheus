package main

import (
	"fmt"
	"os"

	"github.com/misguidedemails/go-octopus-energy"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
)

func metrics(token, mpan, serial string) ([]octopus.Consumption, error) {
	client := octopus.NewClient(token)

	consumption, err := client.ElectricityConsumption(
		mpan,
		serial,
		octopus.ConsumptionRequest{},
	)
	if err != nil {
		return nil, err
	}

	return consumption, nil
}

func pushMetrics(metric octopus.Consumption, address string) error {
	consumptionMetric := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "octopus",
			Subsystem: "electricity",
			Name:      "kilowatthours",
		},
	)

	consumptionMetric.Set(float64(metric.Consumption))

	err := push.New(address, "octopus").
		Collector(consumptionMetric).
		Push()

	if err != nil {
		return err
	}

	return nil
}

func entrypoint() int {
	token := os.Getenv("OCTOPUS_TOKEN")
	mpan := os.Getenv("OCTOPUS_MPAN")
	elecSerial := os.Getenv("OCTOPUS_ELEC_SERIAL")
	pushgateway := os.Getenv("PUSHGATEWAY_ADDRESS")

	if token == "" || mpan == "" || elecSerial == "" || pushgateway == "" {
		fmt.Println("TOKEN, MPAN, PUSHGATEWAY_ADDRESS, or ELEC_SERIAL not given")
		return 1
	}

	metrics, err := metrics(token, mpan, elecSerial)
	if err != nil {
		fmt.Println(err)
		return 1
	}

	err = pushMetrics(metrics[0], pushgateway)
	if err != nil {
		fmt.Println(err)
		return 1
	}

	return 0
}

func main() {
	os.Exit(entrypoint())
}
