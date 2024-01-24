package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/golang/snappy"
	"github.com/go-resty/resty/v2"
	"github.com/gogo/protobuf/proto"
	"github.com/misguidedemails/go-octopus-energy"
	"github.com/prometheus/prometheus/prompb"
)

func pushMetrics(
	metric []octopus.Consumption,
	electricity bool,
) error {
	labels := []prompb.Label{}
	if electricity {
		labels = append(labels, prompb.Label{
			Name: "__name__",
			Value: "octopus_consumption_electricity_kwh",
		})
	} else {
		labels = append(labels, prompb.Label{
			Name: "__name__",
			Value: "octopus_consumption_gas_kwh",
		})
	}

	log.Printf("Pushing %v metrics to pushgateway", len(metric))

	// We can only get consumption in 30 minute intervals, so we need to
	// report the same value for 30 minutes. Otherwise queries in-between
	// will return no data.
	// Frequency determines how frequent the metrics in prometheus will be.
	// frequency := 10 // mins

	client := resty.New().
		SetHeader("Content-Type", "application/x-protobuf").
		SetHeader("X-Prometheus-Remote-Write-Version", "0.1.0").
		SetHeader("Content-Encoding", "snappy").
		SetHeader("User-Agent", "testing/1.1.1.")

	for _, m := range metric {
		log.Printf("Pushing metric for %v, %vkwh", m.IntervalStart, m.Consumption)
		request := prompb.WriteRequest{
			Timeseries: []prompb.TimeSeries{
				{
					Labels: labels,
					Samples: []prompb.Sample{
						{
							Value: float64(m.Consumption),
							Timestamp: m.IntervalStart.UnixMilli(),
						},
					},
				},
			},
		}

		serial, err := proto.Marshal(&request)
		if err != nil {
			return err
		}

		snappySerial := snappy.Encode(nil, serial)

		resp, err := client.R().
			SetBody(snappySerial).
			Post(os.Getenv("OCTOPUS_PUSHGATEWAY"))
		if err != nil {
			return err
		}

		// NOTE: Prometheus has a default limit of ~2 hours for stale data, we will get an
		// out of bounds error if we try to push data older than that.
		// NOTE: Prometheus will reject samples that are older than what it currently has.
		// This means that if we ingested data in the past, and want to backfill further,
		// we will either need to delete the data, or use different labels.

		// TODO: Proper error handling
		if resp.StatusCode() != 204 {
			return fmt.Errorf("Pushgateway returned non-204 status code: %v", resp.StatusCode())
		}
	}

	return nil
}

// TODO: Add backfill length option - how far back to backfill
// TODO: Add frequency option - how frequent should the data be in prometheus
// TODO: Pagination - max 25000 results per page, which is a full year of 30-min intervals
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
		PageSize: 25000,
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

		err = pushMetrics(elecConsumption, true)
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

		err = pushMetrics(gasConsumption, false)
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
