package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/misguidedemails/go-octopus-energy"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
)

func pushMetrics(
	metric octopus.Consumption,
	electricity bool,
) error {
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

	pusher := push.New(os.Getenv("OCTOPUS_PUSHGATEWAY"), "octopus").Collector(consumptionMetric)

	err := pusher.Push()
	if err != nil {
		return err
	}

	return nil
}

func cli(args []string) int {
	ingestGas := flag.Bool("gas", false, "Include gas consumption")
	ingestElec := flag.Bool("electricity", false, "Include electricity consumption")
	flag.Parse()

	if !*ingestGas && !*ingestElec {
		fmt.Println("No consumption type (--gas/--electricity) specified")

		return 1
	}

	requiredVars := []string{"TOKEN", "PUSHGATEWAY"}

	if *ingestGas {
		requiredVars = append(requiredVars, []string{"MPRN", "GAS_SERIAL"}...)
	}

	if *ingestElec {
		requiredVars = append(requiredVars, []string{"MPAN", "ELEC_SERIAL"}...)
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

	if *ingestElec {
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

		err = pushMetrics(elecConsumption[0], true)
		if err != nil {
			fmt.Printf("Error pushing electricity consumption: %v\n", err)

			return 1
		}

		fmt.Println(
			"Pushed elec usage to pushgateway, kWh:", elecConsumption[0].Consumption,
		)
	}

	if *ingestGas {
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

		err = pushMetrics(gasConsumption[0], false)
		if err != nil {
			fmt.Printf("Error pushing gas consumption: %v\n", err)

			return 1
		}

		fmt.Println("Pushed gas usage to pushgateway, kWh:", gasConsumption[0].Consumption)
	}

	return 0
}

func main() {
	os.Exit(cli(os.Args))
}
