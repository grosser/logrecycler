# Logrecycler [![Build Status](https://travis-ci.org/grosser/logrecycler.svg)](https://travis-ci.org/grosser/logrecycler) [![coverage](https://img.shields.io/badge/coverage-100%25-success.svg)](https://github.com/grosser/go-testcov)

Re-process logs from applications you cannot modify to:
- convert plaintext to json
- remove noise
- add log levels / timestamp / details / captured values
- emit prometheus metrics
- emit statsd metrics


## Example

```
stdin: I0530 10:13:00.740596      33 foo.go:132] error connecting to remote host foobar.com:12345
stdout: {"ts":"2020-05-30 10:13:00","level":"error","message":"error connecting to remote host","host":"foobar.com","port":"1234","pattern":"connection-error"}
/metrics: log_total{level="error",host="foobar.com",port="1234",pattern="connection-error"} 1
```


# Setup

## Install

```
curl -sfL <PICK URL FROM RELEASES PAGE> | tar -zx && && chmod +x logrecycler
```

## Configure

Configure a `logrecycler.yaml` in your project root:

```yaml
# optional settings
timestamp_key: ts # what to call the timestamp in the logs (default: no timestamp)
level_key: level # what to call the level in the logs (default: no level)
message_key: msg # what to call the message in the logs (default: message)
preprocess: '[^\]]+\] (?P<message>.*)' # reduce noise from message by replacing it with captured

# enable prometheus /metrics (try to use the same `add` + named captures everywhere)
prometheus_port: 1234 

# enable statsd metrics
statsd_address: 0.0.0.0:8125
statsd_metric: my_app.logs

# patterns to match ... each log line only match the first matching pattern
patterns:
# simple match
- regex: 'error.*parsing' # log line needs to match this
  level: ERROR
  add: # will appear in log and metric
    pattern: parsing-error
# named captures go into logs, replacing message here too
- regex: '(?P<message>error connecting .*) (?P<host>\S+):(?P<port>\d+)'
  level: ERROR
  add:
    pattern: connection-error
# override message if it includes secrets
- regex: 'secret key is'
  level: INFO
  add:
    pattern: secret
    message: secret key redacted # override message
# mark all unmatched as unknown so we can alert on it
- pattern: '' # catch all
  level: WARN
  add:
    pattern: unknown
```

## Use

Pipe your logs to the recycler:

```
<your-program-here> | logrecycler
```
