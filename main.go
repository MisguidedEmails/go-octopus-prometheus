package main

import (
	"flag"
	"fmt"
	// "log" // TODO: something better than fmt.print
	"os"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/misguidedemails/go-octopus-energy"
	"github.com/prometheus/prometheus/prompb"
)

type ingestType string

const (
	ingestTypeElectricity ingestType = "electricity"
	ingestTypeGas         ingestType = "gas"
)

func pushMetrics(
	metric []octopus.Consumption,
	ingest ingestType,
) error {
	labels := []prompb.Label{}

	switch ingest {
	case ingestTypeElectricity:
		labels = append(
			labels,
			prompb.Label{
				Name:  "__name__",
				Value: "octopus_consumption_electricity_kwh",
			},
			prompb.Label{
				Name:  "mpan",
				Value: os.Getenv("OCTOPUS_MPAN"),
			},
			prompb.Label{
				Name:  "serial",
				Value: os.Getenv("OCTOPUS_ELEC_SERIAL"),
			},
		)
	case ingestTypeGas:
		labels = append(
			labels,
			prompb.Label{
				Name:  "__name__",
				Value: "octopus_consumption_gas_kwh",
			},
			prompb.Label{
				Name:  "mprn",
				Value: os.Getenv("OCTOPUS_MPRN"),
			},
			prompb.Label{
				Name:  "serial",
				Value: os.Getenv("OCTOPUS_GAS_SERIAL"),
			},
		)
	}

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

	if os.Getenv("DEBUG") != "" {
		// Yeah I get it, basic auth over HTTP is bad. cool thx.
		client = client.SetDisableWarn(true)
	}

	if os.Getenv("OCTOPUS_PUSHGATEWAY_USER") != "" {
		client = client.SetBasicAuth(
			os.Getenv("OCTOPUS_PUSHGATEWAY_USER"),
			os.Getenv("OCTOPUS_PUSHGATEWAY_PASS"),
		)
	}

	fmt.Printf(
		"Pushing %v %v metrics from %v to %v\n",
		len(metric),
		ingest,
		metric[0].IntervalStart,
		metric[len(metric)-1].IntervalEnd,
	)

	for _, m := range metric {
		// TODO: Debug only?
		// fmt.Printf("Pushing metric for %v, %vkwh\n", m.IntervalStart, m.Consumption)
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
			return fmt.Errorf(
				"Pushgateway returned non-204 status code %v: %v",
				resp.StatusCode(),
				resp.String(),
			)
		}
	}

	return nil
}

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

	return consumption, nil
}

// TODO: Add frequency option - how frequent should the data be in prometheus
func cli(args []string) int {
	ingestGas := flag.Bool("gas", false, "Include gas consumption")
	ingestElec := flag.Bool("electricity", false, "Include electricity consumption")
	// TODO: Accept days/weeks. maybe accept date as an option too.
	since := flag.Duration(
		"since",
		33*24*time.Hour,
		"How long ago to ingest data for",
	)
	fullBackfill := flag.Bool(
		"full-backfill",
		false,
		"Backfill all available data. Overrides --since",
	)

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

	var sinceTime time.Time
	if *fullBackfill {
		sinceTime = time.Date(1972, 1, 1, 0, 0, 0, 0, time.UTC)

		fmt.Println("Backfilling all available data")
	} else {
		sinceTime = time.Now().Add(-*since)
		fmt.Printf("Backfilling from %v\n", sinceTime)
	}

	options := octopus.ConsumptionRequest{
		PeriodFrom: sinceTime,
		OrderBy:    "period", // Oldest first
		PageSize:   1000,     // Around 3 weeks of data per page. Could go higher.
	}

	for _, ingest := range toIngest {
		// Iterate over pages until we get to the end
		for {
			consumption, err := getConsumption(ingest, options)
			if err != nil {
				fmt.Println(err)

				return 1
			}

			if len(consumption) == 0 {
				fmt.Printf("No more %v consumption to ingest\n", ingest)

				break
			}

			err = pushMetrics(consumption, ingest)
			if err != nil {
				fmt.Printf("Error pushing %v consumption: %v\n", ingest, err)

				return 1
			}

			fmt.Printf(
				"Pushed %v usage to pushgateway, kWh: %v\n",
				ingest,
				consumption[0].Consumption,
			)

			// Set the next page "sinceTime" to the last item in the current page
			options.PeriodFrom = consumption[len(consumption)-1].IntervalEnd
		}
	}

	return 0
}

func main() {
	os.Exit(cli(os.Args))
}
