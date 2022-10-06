# Logrecycler [![Build Status](https://travis-ci.org/grosser/logrecycler.svg)](https://travis-ci.org/grosser/logrecycler) [![coverage](https://img.shields.io/badge/coverage-100%25-success.svg)](https://github.com/grosser/go-testcov) [![Build](https://github.com/grosser/logrecycler/workflows/Build/badge.svg)](https://github.com/grosser/logrecycler/releases)

Re-process logs from applications you cannot modify to:
- convert plaintext or glog logs from stdin to json on stdout
- remove noise
- add log levels / timestamp / details / captured values
- emit prometheus metric
- emit statsd metric


## Example

```
stdin: I0530 10:13:00.740596      33 foo.go:132] error connecting to remote host foobar.com:12345
stdout: {"ts":"2020-05-30 10:13:00","level":"error","message":"error connecting to remote host","host":"foobar.com","port":"1234","pattern":"connection-error"}
/metrics: log_total{level="error",host="foobar.com",port="1234",pattern="connection-error"} 1
```


# Setup

## Install

Download [latest binary](https://github.com/grosser/logrecycler/releases):

```
curl -sfL <PICK URL FROM RELEASES PAGE> | tar -zx && chmod +x logrecycler && ./logrecycler --version
```

## Configure

Configure a `logrecycler.yaml` in your project root:

```yaml
# optional settings
timestampKey: ts # what to call the timestamp in the logs (for example @timestamp, ts, leave empty for no timestamp)
levelKey: level # what to call the level in the logs (for example level/lvl/severity, leave empty for no level)
messageKey: msg # what to call the message in the logs (leave empty for 'message')
glog: simple # convert glog style prefix ([IWEF]mmdd hh:mm:ss.uuuuuu threadid file:line] message) into timestamp/level/message
json: simple # assume input starting with `{` and ending with `}` as json and merge it, also set allowMetricLabels to avoid metric spam and match the level+message+timestamp keys with the input
preprocess: '[^\]]+\] (?P<message>.*)' # reduce noise from message by replacing it with captured (for example remove, leave empty for none)
allowMetricLabels: [foo] # ignore everything but these

# enable prometheus /metrics
# when using: try to use the same `add` value and the same named regex captures in patterns below
# to avoid running out of memory
prometheus:
  port: 1234

# enable statsd metric
statsd:
  address: 0.0.0.0:8125
  metric: my_app.logs

# patterns to match ... each log line only match the first matching pattern
patterns:
# simple match
- regex: 'error.*parsing' # log line needs to match this
  level: ERROR
  add: # will appear in log and metric
    pattern: parsing-error # using the same pattern key here, so we can group by pattern when reporting
# named captures go into logs, replacing message here too
- regex: '(?P<message>error connecting .*) (?P<host>\S+):(?P<port>\d+)'
  level: ERROR
  add:
    pattern: connection-error
  ignoreMetricLabels: ["host"] # do not use "host" as metric
# override message if it includes secrets
- regex: 'secret key is'
  level: INFO
  add:
    pattern: secret
    message: secret key redacted # override message
- regex: 'Waited for .* due to client-side throttling'
  level: INFO
  sampleRate: 0.01 # sample only 1%
  add:
    pattern: throttle
# discard spam
- regex: 'todays weather is'
  discard: true
# mark all unmatched as unknown so we can alert on it
- pattern: '' # catch all
  level: WARN
  add:
    pattern: unknown
```

## Use

Pipe your logs to the recycler:

```
set -o pipefail; <your-program-here> | logrecycler
```

# Development

## Test

- `go get github.com/grosser/go-testcov`
- install any version of ruby (used for integration tests)
- `make test`

## Release

Create a new release via github UI, workflow will automatically build a new binary.

## TODO
- `glog: full` to also capture `location` and `thread`
- support json log parsing and rewriting
- basic benchmark of memory/cpu overhead (without counting startup time)
- more examples


# Author
[Michael Grosser](http://grosser.it)<br/>
michael@grosser.it<br/>
License: MIT<br/>
