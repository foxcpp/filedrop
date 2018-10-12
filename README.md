# filedrop
Too lightweight file storage server with HTTP API.

### HTTP API

POST single file to `/filedrop` to save it. 
For example:
```
POST /filedrop/screenshot.png
Content-Type: image/png
Content-Length: XXXX
```

You will get response with full file URL, like this one:
```
http://example.com/filedrop/41a8f78c-ce06-11e8-b2ed-b083fe9824ac/screenshot.png
```

Actually only UUID is used for file access. So you can change last part of URL 
how you like:
```
http://example.com/filedrop/41a8f78c-ce06-11e8-b2ed-b083fe9824ac/amazing-screenshot.png
```

You can specify `max-uses` and `store-time-secs` to override default settings
from server configuration (however you can't set value higher then configured).

```
POST /filedrop/screenshot.png?max-uses=5&store-time-secs=3600
```
Following request will store file screenshot.png for one hour (3600 seconds)
and allow it to be downloaded not more than 10 times.

### Authorization

filedrop supports very basic access control. Basically, it can execute SQL
query with contents of `Authorization` header and file name. If query returns 1
- access will be allowed, if query returns 0 or fails - client will get 403.

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