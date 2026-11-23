# Ho Chi Minh City career sources

The Job Lake worker polls public SmartRecruiters postings for Bosch, Grab,
Avery Dennison, Accor, SGS, and Eurofins. The worker queries Vietnamese
postings, checks the listed city, reads job detail, and publishes normalized
active jobs to the durable RabbitMQ queue `job.new`. The existing HunterJob
matcher consumes that queue.

```sh
go run ./cmd/joblake -preview
go run ./cmd/joblake -once
go run ./cmd/joblake
```

Preview emits newline-delimited JSON and does not connect to RabbitMQ.
The regular worker syncs on startup and waits six hours between successful
runs. Use `-interval` to change the polling interval or `-once` in an
external scheduler. In production, set `RABBITMQ_URL` to the same broker
used by the matcher. Each job event is persistent and broker-confirmed.

To replace the sources, pass `-sources /path/to/sources.json`. Records use
`{"company":"Display name","board":"SmartRecruitersBoardIdentifier"}`.
The default registry is embedded from `internal/joblake/sources.json`.

The event ID is derived from the board and upstream job ID. The worker
emits the current active snapshot every run so new preferences can match
existing postings. Existing matches are deduplicated by the matcher and
retain their previous user status. This worker does not infer missing
experience ranges or publish closure events from missing postings.
