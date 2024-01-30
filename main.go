package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/go-resty/resty/v2"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
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
			Name:  "__name__",
			Value: "octopus_consumption_electricity_kwh",
		})
	} else {
		labels = append(labels, prompb.Label{
			Name:  "__name__",
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
							Value:     float64(m.Consumption),
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

type ingestType string

const (
	ingestTypeGas         ingestType = "gas"
	ingestTypeElectricity ingestType = "electricity"
)

// Ingest data from Octopus API into Prometheus.
func getConsumption(
	ingest ingestType,
	options octopus.ConsumptionRequest,
) (consumption []octopus.Consumption, err error) {
	client := octopus.NewClient(os.Getenv("OCTOPUS_TOKEN"))

	switch ingest {
	case ingestTypeGas:
		consumption, err = client.GasConsumption(
			os.Getenv("OCTOPUS_MPRN"),
			os.Getenv("OCTOPUS_GAS_SERIAL"),
			options,
		)
	case ingestTypeElectricity:
		consumption, err = client.ElectricityConsumption(
			os.Getenv("OCTOPUS_MPAN"),
			os.Getenv("OCTOPUS_ELEC_SERIAL"),
			options,
		)
	}

	// TODO: beter handling
	if err != nil {
		return nil, err
	}

	if len(consumption) == 0 {
		return nil, fmt.Errorf("No consumption found")
	}

	return consumption, nil
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

	toIngest := []ingestType{}

	requiredVars := []string{"TOKEN", "PUSHGATEWAY"}

	if *ingestGas {
		requiredVars = append(requiredVars, []string{"MPRN", "GAS_SERIAL"}...)
		toIngest = append(toIngest, ingestTypeGas)
	}

	if *ingestElec {
		requiredVars = append(requiredVars, []string{"MPAN", "ELEC_SERIAL"}...)
		toIngest = append(toIngest, ingestTypeElectricity)
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

	options := octopus.ConsumptionRequest{
		PageSize: 25000,
	}

	for _, ingest := range toIngest {
		consumption, err := getConsumption(ingest, options)
		if err != nil {
			fmt.Println(err)

			return 1
		}

		err = pushMetrics(consumption, ingest == ingestTypeElectricity)
		if err != nil {
			fmt.Printf("Error pushing %v consumption: %v\n", ingest, err)

			return 1
		}

		fmt.Printf(
			"Pushed %v usage to pushgateway, kWh: %v\n",
			ingest,
			consumption[0].Consumption,
		)
	}

	return 0
}

func main() {
	os.Exit(cli(os.Args))
}
