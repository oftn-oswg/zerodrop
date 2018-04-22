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

## Blacklist

The blacklist syntax is similar to that of [gitignore][1].

An optional prefix `!` which negates the pattern can be used for whitelisting. For example to only allow from local:

```
*      # First blacklist everyone
! ::1  # Allow localhost
```

An optional prefix `@` is for targeted geofencing, i.e., `@ lat lng (optional radius)`. The default radius is 25m. For example to block Atlantic City:

```
@ 39.377297 -74.451082 (7km)
```

| Unit      | Symbol |
| --------- | ------ |
| meter     | m      |
| kilometer | km     |
| mile      | mi     |
| feet      | ft     |

An optional prefix `~` indicates a hostname regular expression match.

```
shady.com
~ (.*)\.shady\.com # Block subdomains of shady
```

Use CIDR notation to denote a network: `192.168.2.0/24`
Or just list the IP address to block: `192.168.2.6`
Or just use a simple hostname match: `www.google.com`

[1]: https://git-scm.com/docs/gitignore
