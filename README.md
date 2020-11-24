# go-octopus-prometheus

Pull current gas and electricity usage from the Octopus Energy API and then push those results to Prometheus PushGateway.

This is intended to be ran on a cron, which is why we use Pushgateway and not as a long-lived application exposing these metrics itself.

## Usage

You'll need to have the following environment variables set:

- `OCTOPUS_TOKEN`: API Token for the Octopus API
- `OCTOPUS_MPAN`: Electricity meter MPAN
- `OCTOPUS_ELEC_SERIAL`: Serial of the electricity meter you want to use
- `OCTOPUS_MPRN`: Gas meter MPRN
- `OCTOPUS_GAS_SERIAL`: Serial of the gas meter
- `OCTOPUS_PUSHGATEWAY`: Address to the pushgateway endpoint

### K8s cron job

As my planned usage is to run this in Kubernetes as a cron job, there is an example K8s config in `k8s-example-cron.yml`

## Metrics

We publish the following metrics to prometheus:

- `octopus_gas_kilowatthours`: kWh from the gas meter
- `octopus_electricity_kilowatthours`: kWh from the electricity meter
