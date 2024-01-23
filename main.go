package main

import (
	"fmt"
	"os"

	"github.com/misguidedemails/go-octopus-energy"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
)

func pushMetrics(elec, gas prometheus.Collector, address string) error {
	pusher := push.New(address, "octopus").Collector(elec)
	pusher.Collector(gas)

	err := pusher.Push()
	if err != nil {
		return err
	}

	return nil
}

func createGauge(
	metric octopus.Consumption,
	electricity bool,
) prometheus.Gauge {
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

	return consumptionMetric
}

func entrypoint() int {
	requiredVars := []string{
		"TOKEN", "MPAN", "MPRN", "ELEC_SERIAL", "GAS_SERIAL", "PUSHGATEWAY",
	}

	var missingVars []string

	for _, env := range requiredVars {
		varName := fmt.Sprintf("OCTOPUS_%s", env)

		if value := os.Getenv(varName); value == "" {
			missingVars = append(missingVars, varName)
		}
	}

	if len(missingVars) > 0 {
		fmt.Println("Missing ENV vars:", missingVars)

		return 1
	}

	client := octopus.NewClient(os.Getenv("OCTOPUS_TOKEN"))

	options := octopus.ConsumptionRequest{
		PageSize: 1,
	}

	elecConsumption, err := client.ElectricityConsumption(
		os.Getenv("OCTOPUS_MPAN"),
		os.Getenv("OCTOPUS_ELEC_SERIAL"),
		options,
	)
	if err != nil {
		fmt.Println(err)

		return 1
	}

	if len(elecConsumption) == 0 {
		fmt.Println("No electricity consumption found")

		return 1
	}

	elecGauge := createGauge(elecConsumption[0], true)

	gasConsumption, err := client.GasConsumption(
		os.Getenv("OCTOPUS_MPRN"),
		os.Getenv("OCTOPUS_GAS_SERIAL"),
		options,
	)
	if err != nil {
		fmt.Println(err)

		return 1
	}

	if len(gasConsumption) == 0 {
		fmt.Println("No gas consumption found")

		return 1
	}

	gasGauge := createGauge(gasConsumption[0], false)

	err = pushMetrics(
		gasGauge,
		elecGauge,
		os.Getenv("OCTOPUS_PUSHGATEWAY"),
	)
	if err != nil {
		fmt.Println(err)

		return 1
	}

	fmt.Println("Pushed gas usage to pushgateway, kWh:", gasConsumption[0].Consumption)
	fmt.Println(
		"Pushed elec usage to pushgateway, kWh:",
		elecConsumption[0].Consumption,
	)

	return 0
}

func main() {
	os.Exit(entrypoint())
}
