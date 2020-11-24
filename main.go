package main

import (
	"fmt"
	"os"

	"github.com/misguidedemails/go-octopus-energy"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
)

func pushMetrics(metric octopus.Consumption, address string, electricity bool) error {
	var consumptionMetric prometheus.Gauge
	if electricity {
		consumptionMetric = prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "octopus",
				Subsystem: "electricity",
				Name:      "kilowatthours",
			},
		)
	} else {
		consumptionMetric = prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "octopus",
				Subsystem: "gas",
				Name:      "kilowatthours",
			},
		)
	}

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
	mprn := os.Getenv("OCTOPUS_MPRN")
	elecSerial := os.Getenv("OCTOPUS_ELEC_SERIAL")
	gasSerial := os.Getenv("OCTOPUS_GAS_SERIAL")
	pushgateway := os.Getenv("PUSHGATEWAY_ADDRESS")

	if token == "" || mpan == "" || elecSerial == "" || pushgateway == "" {
		fmt.Println("TOKEN, MPAN, PUSHGATEWAY_ADDRESS, or ELEC_SERIAL not given")

		return 1
	}

	client := octopus.NewClient(token)

	options := octopus.ConsumptionRequest{
		PageSize: 1,
	}

	elecConsumption, err := client.ElectricityConsumption(mpan, elecSerial, options)
	if err != nil {
		fmt.Println(err)

		return 1
	}

	err = pushMetrics(elecConsumption[0], pushgateway, true)
	if err != nil {
		fmt.Println(err)

		return 1
	}

	gasConsumption, err := client.GasConsumption(mprn, gasSerial, options)
	if err != nil {
		fmt.Println(err)

		return 1
	}

	err = pushMetrics(gasConsumption[0], pushgateway, false)
	if err != nil {
		fmt.Println(err)

		return 1
	}

	return 0
}

func main() {
	os.Exit(entrypoint())
}
