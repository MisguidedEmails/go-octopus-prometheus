package main

import (
	"fmt"
	"os"

	"github.com/misguidedemails/go-octopus-energy"
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


func entrypoint() int {
	token := os.Getenv("OCTOPUS_TOKEN")
	mpan := os.Getenv("OCTOPUS_MPAN")
	elecSerial := os.Getenv("OCTOPUS_ELEC_SERIAL")

	if token == "" || mpan == "" || elecSerial == "" {
		fmt.Println("TOKEN, MPAN, or ELEC_SERIAL not given")
		return 1
	}

	_, err := metrics(token, mpan, elecSerial)
	if err != nil {
		fmt.Println(err)
		return 1
	}

	return 0
}


func main() {
	os.Exit(entrypoint())
}
