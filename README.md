# octopus-prometheus

Retrieve usage from Octopus Energy and push to Prometheus-compatible RemoteWrite endpoint.

Due to the delayed nature of the consumption data from Octopus, this is intended to be ran on a cron. The data is then backfilled via RemoteWrite.

## Usage

You'll need to have the following environment variables set:

- `OCTOPUS_TOKEN`: For the Octopus API
- `OCTOPUS_MPAN`: For the electicity meter
- `OCTOPUS_ELEC_SERIAL`: Serial of the electricity meter
- `OCTOPUS_MPRN`: For the gas meter
- `OCTOPUS_GAS_SERIAL`: Serial of the gas meter
- `OCTOPUS_REMOTE_WRITE`: Address to the remote write endpoint

If using basic auth for prometheus:

- `OCTOPUS_REMOTE_WRITE_USER`
- `OCTOPUS_REMOTE_WRITE_PASS`

### CLI

```bash
# Ingest both gas and electricity usage, for the past week
octopus-prometheus --gas --electricity --since 168h  # default is 792h (4 weeks)

# Ingesting all available historic data for electricity (recommended to run once, then recurringly with --since, if you want full history)
octopus-prometheus --electricity --full-backfill

# Usage
octopus-prometheus --help
```

### K8s cron job

As my planned usage is to run this in Kubernetes as a cron job, there is an example K8s config in `k8s-example-cron.yml`

## Metrics

The following metrics are published:

- `octopus_consumption_gas_kwh`: kWh from the gas meter
  - labels: `mprn`, `serial`
- `octopus_consumption_electricity_kwh`: kWh from the electricity meter
  - labels: `mpan`, `serial`

## Considerations

Retention must be configured to include the period you wish to backfill. You may want a specific "long-term" instance to store this data, separate from any regular monitoring instance.

Victoria Metrics (recommended):

- By default has support for backfilling data anywhere within its retention period (it will accept older data, but then immediately remove it)
- Handles wide time ranges much better than Prometheus (at least compared to the default UI).
  - This is presumably because Victoria Metrics will automatically adjust query resolution based on the time range, but the basic Prometheus UI does not.
- Allows ingesting samples in any order - running this on a cron is not an issue, as any existing samples will be accepted.

Prometheus:

- Has quirks around backfilling
  - Prometheus will not accept data that is older than the current time minus the [`storage.tsdb.out_of_order_time_window`](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#tsdb) (experimental) value. Seems to be around 24 hours by default.
  - Does not support backfilling a sample that is older than the most recent sample it has. e.g. if the latest sample is 2024-01-20 19:00, then you cannot ingest samples, that have the same labels, before this time. This makes it awkward to run this on a cron, as there might be an overlap. Though, we could also handle that a bit better (i.e. storing the last time we pushed, querying for it, or ignoring any "out of order" errors).
- Is slower at ingesting 40k+ samples in a single run (i.e. `--full-backfill`)
- Maybe not recommended to set the out-of-order time window while also using Prometheus normally - prometheus wasn't designed with this in mind, and buggy client-side timestamps - if the instance is being used for regular monitoring - may then be accepted and cause issues.

## TODO

- [ ] Option to "Ammortise" the usage over each 30 minute period - since consumption is reported in 30 minute intervals, any prometheus queries between these intervals will not show. Though overall usage will be correct, e.g. querying over a period of a few hours. We could ammortise the usage so that it reports every x seconds, instead of every 30 minutes, but reducing the kWh value appropriately so that if it's sum'd over the 30 minute period, it's the same as the original value.
  - Allows better querying and aggregation of data. Can query cross 30 minute boundaries, without huge swings of usage.
  - Prevents empty queries when quering in-between 30 minute intervals.
  - Need to ensure that this does not present an inaccurate view of the data - maybe a different metric name
- [ ] Add tests - simple integration tests to ensure pushing is functioning
- [ ] Hand-holding Prometheus - e.g. resuming from where we left off last time, so prometheus doesn't get mad
- [ ] Cleanup all code TODOs
