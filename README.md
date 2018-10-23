filedrop
==========

[![Travis CI](https://img.shields.io/travis/com/foxcpp/filedrop.svg?style=flat-square&logo=Linux)](https://travis-ci.com/foxcpp/filedrop)
[![CodeCov](https://img.shields.io/codecov/c/github/foxcpp/filedrop.svg?style=flat-square)](https://codecov.io/gh/foxcpp/filedrop)
[![Issues](https://img.shields.io/github/issues-raw/foxcpp/filedrop.svg?style=flat-square)](https://github.com/foxcpp/filedrop/issues)
[![License](https://img.shields.io/github/license/foxcpp/filedrop.svg?style=flat-square)](https://github.com/foxcpp/filedrop/blob/master/LICENSE)

Too lightweight file storage server with HTTP API.

**Currently filedrop is implemented only as a library. Sections below also
document ideas for standalone server. See issue #3.**

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

filedrop supports very basic access control. Basically, it can execute SQL
query with contents of `Authorization` header and file name. If query returns 1 - access 
will be allowed, if query returns 0 or fails - client will get 403.

Also if you are using filedrop as a library, you can instead just pass custom 
authorization callback. 

See [configuration example](filedrop.example.yml) for details.

### Installation

`go get` or clone this repo and build binary from `filedropd` package. Pass
path to configuration in first argument. You are perfect.

### Configuration

[Documented example](filedrop.example.yml) is included in repo, check it out.

### Embedding

If your backend server is written in Golang or you are not a big fan of
microservices architecture - you can run filedrop server as part of your
program. See [documentation](godocs.org/github.com/foxcpp/filedrop) for
details.
