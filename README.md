# Simple Go endpoint for proxying steam group member lists

This currently is intended for execution on a [fly.io](https://fly.io) free-tier VM.

## To setup:
Setup an account on [fly.io](https://fly.io) then install `flyctl`.
```sh
$ cd memberproxy-go
$ flyctl launch
...
$ flyctl secrets set GROUP=Valve # steam group example name to query
$ flyctl secrets set WEBHOOKURL=https://discord.com/api/webhooks/1/2 # Discord webhook to log New/Removed group members to
$ flyctl secrets set SECRETENDPOINT=02938740958723894752345 # use something random like this lol
```
And don't forget to point a domain to the VMs IP (`flyctl ips list`) (if you want).
