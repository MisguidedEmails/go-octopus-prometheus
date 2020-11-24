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

	err = pushMetrics(elecConsumption[0], os.Getenv("OCTOPUS_PUSHGATEWAY"), true)
	if err != nil {
		fmt.Println(err)

		return 1
	}
	fmt.Println(
		"Pushed elec usage to pushgateway, kWh:",
		elecConsumption[0].Consumption,
	)

	gasConsumption, err := client.GasConsumption(
		os.Getenv("OCTOPUS_MPRN"),
		os.Getenv("OCTOPUS_GAS_SERIAL"),
		options,
	)
	if err != nil {
		fmt.Println(err)

		return 1
	}

	err = pushMetrics(gasConsumption[0], os.Getenv("OCTOPUS_PUSHGATEWAY"), false)
	if err != nil {
		fmt.Println(err)

		return 1
	}

	fmt.Println("Pushed gas usage to pushgateway, kWh:", gasConsumption[0].Consumption)

	return 0
}

func main() {
	os.Exit(entrypoint())
}
