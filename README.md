filedrop
==========

[![Travis CI](https://img.shields.io/travis/com/foxcpp/filedrop.svg?style=flat-square&logo=Linux)](https://travis-ci.com/foxcpp/filedrop)
[![CodeCov](https://img.shields.io/codecov/c/github/foxcpp/filedrop.svg?style=flat-square)](https://codecov.io/gh/foxcpp/filedrop)
[![Issues](https://img.shields.io/github/issues-raw/foxcpp/filedrop.svg?style=flat-square)](https://github.com/foxcpp/filedrop/issues)
[![License](https://img.shields.io/github/license/foxcpp/filedrop.svg?style=flat-square)](https://github.com/foxcpp/filedrop/blob/master/LICENSE)

Lightweight file storage server with HTTP API.

### Features
- Painless configuration! You don't even have to `rewrite` requests on your reverse proxy!
- Limits support! Link usage count, file size and storage time.
- Embeddable! Can run as part of your application.

You can use filedrop either as a standalone server or as a part of your application.
In former case you want to check `filedropd` subpackage, in later case just
import `filedrop` package and pass config stucture to `filedrop.New`, returned
object implements `http.Handler` so you can use it how you like.

### Installation

This repository uses Go 1.11 modules. Things may work with old `GOPATH`
approach but we don't support it so don't report cryptic compilation errors
caused by wrong dependency version.

`master` branch contains code from latest (pre-)release. `dev` branch
contains bleeding-edge code. You probably want to use one of [tagged
releases](https://github.com/foxcpp/filedrop/releases).

#### SQL drivers

filedrop uses SQL database as a meta-information storage so you need a
SQL driver for it to use.

When building standalone server you may want to enable one of the
supported SQL DBMS using build tags:
* `postgres` for PostgreSQL
* `sqlite3` for SQLite3
* `mysql` for MySQL

**Note:** No MS SQL Server support is planned. However if you would like
to see it - PRs are welcome.

When using filedrop as a library you are given more freedom. Just make
sure that you import driver you use.

#### Library

Just use `github.com/foxcpp/filedrop` as any other library. Documentation
is here: [godoc.org](https://godoc.org/github.com/foxcpp/filedrop).

#### Standalone server

See `fildropd` subdirectory. To start server you need a configuration
file. See example [here](filedrop.example.yml). It should be pretty
straightforward. Then just pass path to configuration file in
command-line arguments.

```
filedropd /etc/filedropd.yml
```

systemd unit file is included for your convenience.

### HTTP API

POST single file to any endpoint to save it.
For example:
```
POST /filedrop
Content-Type: image/png
Content-Length: XXXX
```

You will get response with full file URL (endpoint used to POST + UUID), like this one:
```
http://example.com/filedrop/41a8f78c-ce06-11e8-b2ed-b083fe9824ac
```

You can add anything as last component to URL to give it human-understandable meaning:
```
http://example.com/filedrop/41a8f78c-ce06-11e8-b2ed-b083fe9824ac/amazing-screenshot.png
```
However you can't add more than one component:
```
http://example.com/filedrop/41a8f78c-ce06-11e8-b2ed-b083fe9824ac/invalid/in/filedrop
```

You can specify `max-uses` and `store-time-secs` to override default settings
from server configuration (however you can't set value higher then configured).

```
POST /filedrop/screenshot.png?max-uses=5&store-secs=3600
```
Following request will store file screenshot.png for one hour (3600 seconds)
and allow it to be downloaded not more than 10 times.

**Note** To get `https` scheme in URLs downstream server should set header
`X-HTTPS-Downstream` to `1` (or you can also set HTTPSDownstream config option)

### Authorization

When using filedrop as a library you can setup custom callbacks
for access control.

See `filedrop.AuthConfig` documentation.
