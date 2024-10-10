# stress.py

stress.py is a tool used to stress test Loki. It supports a custom amount of log lines per second and additionally supports multiple processes, which in the case of OTEL/Loki produces multiple streams.

Our pipeline during this tool's usage was: opentelemetry-collector -> opentelemetry-collector -> kafka -> opentelemetry-collector -> Loki

You can probably use it for other things like ELK, it's pretty simple.

### usage

Grab the [log-stressor](https://github.com/ViaQ/cluster-logging-collector-benchmarks/tree/main/go/log-stressor) code and build a binary with:
```env GOOS=linux GOARCH=amd64 go build -o log-stressor```

It probably supports other tools for this purpose, feel free to try stress-ng or whatever you like. That's all we've used it with

You'll need to modify two variables in the script for usage:
* `LOG_DIR` - this will be the path to your logs
* `LOG_STRESSOR` - this is the path to the binary you built above

Optionally, you can also modify `TRUNCATE_SIZE` to truncate the log files once they reach a certain size (1GB by default). The current/default configuration to truncate to is 50MB. `SIZE_LIMIT` is defaulted to 1GB per file (when truncation occurs), but can be modified as needed.

Once all of that is done, go ahead and run it.
```
./stress.py <msgpersec> [processes]
```

Example:
```
./stress.py 100 10
```
This will create 10 log files (0-9) with 100 messages per second (1000 logs/sec). It's capable of much more than that, in my testing we got up to 200k logs/sec through our pipeline but I'm sure it could go higher. 
