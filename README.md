# Zerodrop 🕵️

A utility for private redirects and proxies that disappear after being used once. 

## Features

* Web interface for creating resources
* Create proxies and redirections for a given URL
* Upload files or manually enter content in a textarea
* Access control
* Expire access to a resource after number of downloads
* Block or allow access based on IP address
* Block or allow access based on IP network
* Block or allow access based on GeoIP location
* Block or allow access based on hostname matching (w/ regex)
* Publish "secret" pages with UUID generation
* Self-destruct action which removes and kills running binary; configurable.

## Blacklist

The blacklist syntax is similar to that of [gitignore][1]. An optional prefix `!` which negates the pattern can be used for whitelisting.

### Categories

1. Match All [`*`] (Useful for creating a whitelist)
2. Match IP [e.g. `203.0.113.6` or `2001:db8::68`]
3. Match IP Network [e.g.: `192.0.2.0/24` or `::1/128`]
4. Match Hostname [e.g. `crawl-66-249-66-1.googlebot.com`]
5. Match Hostname RegExp [e.g.: `~ .*\.cox\.net`]
6. Match Geofence [e.g.: `@ 39.377297 -74.451082 (7km)`]
7. Match [database][2] [e.g. `db datacenters` or `db tor`]

### Whitelist

For example to only allow from local:

```
# This strange blacklist only allows access from localhost and google bots
*
! ::1  # Allow localhost
! ~ .*\.google(bot)?\.com$
```

### Geofencing

A `@` prefix is for targeted geofencing, i.e., `@ lat lng (optional radius)`. The default radius is 25m. For example to block Atlantic City:

```
@ 39.377297 -74.451082 (7km)
```

| Unit      | Symbol |
| --------- | ------ |
| meter     | m      |
| kilometer | km     |
| mile      | mi     |
| feet      | ft     |

### Regular Expression

A `~` prefix indicates a hostname regular expression match.

```
shady.com
~ (.*)\.shady\.com # Block subdomains of shady
```

## Databases

A rule that begins with "`db `" will be matched with a database by name, e.g.,
`!db tor` to whitelist Tor exit nodes. The database file must be specified in
the config.

```yaml
ipcat:
    cloudflare: cloudflare.csv
    datacenters: datacenters.csv
    tor: torexitnodes.csv
```

The format of the CSV file is specified by [ipcat][2] rules.


[1]: https://git-scm.com/docs/gitignore
[2]: https://github.com/oftn-oswg/ipcat
