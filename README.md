notifications-push
==================

[![CircleCI](https://circleci.com/gh/Financial-Times/notifications-push.svg?style=svg)](https://circleci.com/gh/Financial-Times/notifications-push) [![Go Report Card](https://goreportcard.com/badge/github.com/Financial-Times/notifications-push)](https://goreportcard.com/report/github.com/Financial-Times/notifications-push) [![Coverage Status](https://coveralls.io/repos/github/Financial-Times/notifications-push/badge.svg)](https://coveralls.io/github/Financial-Times/notifications-push)

Notifications-push is a microservice that provides push notifications of publications or changes of articles and lists of content.
The microservice consumes a specific Apache Kafka topic group, then it pushes a notification for each article or list available in the consumed Kafka messages.

How to Build & Run the binary
-----------------------------

1. Install, build and test:
```
curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
go get -u github.com/Financial-Times/notifications-push
cd $GOPATH/src/github.com/Financial-Times/notifications-push

dep ensure -vendor-only
go test ./... -race
go install
```
2. Run locally:

* Create tunnel to the Kafka service inside the cluster for ports 2181 and 9092 (use the public IP):
```
ssh -L 2181:localhost:2181 -L 9092:localhost:9092 username@<host>
```

* Add the private DNS of the Kafka machine to the hosts file:
```
127.0.0.1       <private_dns> 
```

* Start the service using environment variables:

```
export NOTIFICATIONS_RESOURCE=content \
    && export KAFKA_ADDRS=localhost:2181 \
    && export GROUP_ID=notifications-push-yourtest \
    && export TOPIC=PostPublicationEvents \
    && export NOTIFICATIONS_DELAY=10 \
    && export API_BASE_URL="http://api.ft.com" \
    && export CONTENT_TYPE_WHITELIST="application/vnd.ft-upp-article+json,application/vnd.ft-upp-content-package+json" \
    && export CONTENT_URI_WHITELIST="^http://(methode|wordpress|content)-(article|collection|content-placeholder)-(transformer|mapper|unfolder)(-pr|-iw)?(-uk-.*)?\\.svc\\.ft\\.com(:\\d{2,5})?/(content)/[\\w-]+.*$" \
    && ./notifications-push
```

* or via command-line parameters:

```
./notifications-push \
    --notifications_resource="content" \
    --consumer_addr="localhost:2181" \
    --consumer_group_id="notifications-push" \
    --topic="PostPublicationEvents" \
    --notifications_delay=10 \
    --api-base-url="http://api.ft.com" \
    --api_key_validation_endpoint="t800/a" \
    --content_type_whitelist="application/vnd.ft-upp-article+json,application/vnd.ft-upp-content-package+json" \
    --content_uri_whitelist="^http://(methode|wordpress|content)-(article|collection|content-placeholder)-(transformer|mapper|unfolder)(-pr|-iw)?(-uk-.*)?\\.svc\\.ft\\.com(:\\d{2,5})?/(content)/[\\w-]+.*$"
```

NB: for the complete list of options run `./notifications-push -h`

HTTP endpoints
----------
```curl -i --header "x-api-key: «api_key»" https://api.ft.com/content/notifications-push```

The following content types could be also specified for which the client would like to receive notifications by setting a "type" parameter on the request: `Article`, `ContentPackage`, `Audio` and `All` to include everything (also CPHs).
If not specified, by default `Article` is used. If an invalid type is requested an HTTP 400 Bad Request is returned.

E.g.
```curl -i --header "x-api-key: «api_key»" https://api.ft.com/content/notifications-push?type=Article```

### Filter DELETE messages by type
When a content has been deleted (`http://www.ft.com/thing/ThingChangeType/DELETE`), the kafka payload is empty and we cannot extract the content type from the message. In this case, there are 2 possible behaviours:
- if the `Content-Type` header is empty or equal to `application-json`, then we cannot resolve the content type of the message. Therefore, the notification will be sent to all the subscribers despite their accepted content type.

- if the `Content-Type` header has been specified and it is not equal to `application-json`, then we can resolve the content type based on this header.

e.g. a DELETE message with header `Content-Type: application/vnd.ft-upp-audio+json` will be considered an Audio content. Therefore, the notification will be sent to those subscribers who accept `type=Audio` or `type=All`.


### Push stream

By opening a HTTP connection with a GET method to the `/{resource}/notifications-push` endpoint, subscribers can consume the notifications push stream for the resource specified in the configuration (content or lists).
The stream will look like the following one.

```
data: []

data: []

data: [{"apiUrl":"http://api.ft.com/content/648bda7b-1187-3496-b48e-57ecb14d5b0a","id":"http://www.ft.com/thing/648bda7b-1187-3496-b48e-57ecb14d5b0a","type":"http://www.ft.com/thing/ThingChangeType/UPDATE","title":"For Ioana & Ioana only;","standout":{"scoop":true}}]

data: []

data: []

data: []

data: [{"apiUrl":"http://api.ft.com/content/e2e49a44-ef3c-11e5-aff5-19b4e253664a","id":"http://www.ft.com/thing/e2e49a44-ef3c-11e5-aff5-19b4e253664a","type":"http://www.ft.com/thing/ThingChangeType/UPDATE","title":"For Ioana 2","standout":{"scoop":false}}]

data: []

data: [{"apiUrl":"http://api.ft.com/content/d38489fa-ecf4-11e5-888e-2eadd5fbc4a4","id":"http://www.ft.com/thing/d38489fa-ecf4-11e5-888e-2eadd5fbc4a4","type":"http://www.ft.com/thing/ThingChangeType/UPDATE","title":"For Ioana 3","standout":{"scoop":false}}]

data: [{"apiUrl":"http://api.ft.com/content/648bda7b-1187-3496-b48e-57ecb14d5b0a","id":"http://www.ft.com/thing/648bda7b-1187-3496-b48e-57ecb14d5b0a","type":"http://www.ft.com/thing/ThingChangeType/UPDATE","title":"For Ioana 4","standout":{"scoop":false}}]

data: [{"apiUrl":"http://api.ft.com/content/648bda7b-1187-3496-b48e-57ecb14d5b0a","id":"http://www.ft.com/thing/648bda7b-1187-3496-b48e-57ecb14d5b0a","type":"http://www.ft.com/thing/ThingChangeType/UPDATE","title":"For Ioana 4","standout":{"scoop":false}}]
```

The empty `[]` lines are heartbeats. Notifications-push will send a heartbeat every 30 seconds to keep the connection active.

The notifications-push stream endpoint allows a `monitor` query parameter. By setting the `monitor` flag as `true`, the push stream returns `publishReference` and `lastModified` attributes in the notification message, which is necessary information for UPP internal monitors such as [PAM](https://github.com/Financial-Times/publish-availability-monitor).


To test the stream endpoint you can run the following CURL commands :
```
curl -X GET "http://localhost:8080/content/notifications-push"
curl -X GET "http://localhost:8080/content/notifications-push?monitor=true"
curl -X GET "https://<user>@<password>:pre-prod-up.ft.com/content/notifications-push"
```

**WARNING: In CoCo, this endpoint does not work under `/__notifications-push/`.**
The reason for this is because Vulcan does not support long polling of HTTP requests. We worked around this issue by forwarding messages through Varnish to a fixed port for both services.

**Productionizing Push API:**
The API Gateway does not support long polling of HTTP requests, so the requests come through Fastly. Everytime a client tries to connect to Notifications Push, the service performs a call to the API Gateway in order to validate the API key from the client.

### Notification history
A HTTP GET to the `/__history` endpoint will return the history of the last notifications consumed from the Kakfa queue.
The expected payload should look like the following one:

```
[
	{
		"apiUrl": "http://api.ft.com/content/eabefe3e-a4b9-11e6-8b69-02899e8bd9d1",
		"id": "http://www.ft.com/thing/eabefe3e-a4b9-11e6-8b69-02899e8bd9d1",
		"type": "http://www.ft.com/thing/ThingChangeType/UPDATE",
		"publishReference": "tid_jwhfe7n6dj",
		"lastModified": "2016-11-07T13:59:44.950Z"
	},
	{
		"apiUrl": "http://api.ft.com/content/c92d7b0c-a4c7-11e6-8b69-02899e8bd9d1",
		"id": "http://www.ft.com/thing/c92d7b0c-a4c7-11e6-8b69-02899e8bd9d1",
		"type": "http://www.ft.com/thing/ThingChangeType/UPDATE",
		"publishReference": "tid_owd5zqw11m",
		"lastModified": "2016-11-07T13:59:04.546Z"
	},
	{
		"apiUrl": "http://api.ft.com/content/58437fc0-a4f1-11e6-8898-79a99e2a4de6",
		"id": "http://www.ft.com/thing/58437fc0-a4f1-11e6-8898-79a99e2a4de6",
		"type": "http://www.ft.com/thing/ThingChangeType/DELETE",
		"publishReference": "tid_5z8dxzsesj",
		"lastModified": "2016-11-07T13:58:36.285Z"
	}
]
```

### Stats
A HTTP GET to the `/__stats` endpoint will return the stats about the current subscribers that are consuming the notifications push stream
The expected payload should look like the following one:
```
{
	"nrOfSubscribers": 2,
	"subscribers": [
		{
			"addr": "127.0.0.1:61047",
			"since": "Nov  7 14:26:04.018",
			"connectionDuration": "2m41.693365011s",
			"type": "dispatcher.standardSubscriber"
		},
		{
			"addr": "192.168.1.3:65345",
			"since": "Nov  7 14:26:06.259",
			"connectionDuration": "2m39.453175004",
			"type": "dispatcher.monitorSubscriber"
		}
	]
}
```

How to Build & Run with Docker
------------------------------
```
    docker build -t coco/notifications-push .

    docker run --env NOTIFICATIONS_RESOURCE=content \
        --env KAFKA_ADDRS=localhost:2181 \
        --env GROUP_ID="notifications-push-yourtest" \
        --env TOPIC="PostPublicationEvents" \
        --env NOTIFICATIONS_DELAY=10 \
        --env API_BASE_URL="http://api.ft.com" \
        --env CONTENT_TYPE_WHITELIST="application/vnd.ft-upp-article+json,application/vnd.ft-upp-content-package+json" \
        --env CONTENT_URI_WHITELIST="^http://(methode|wordpress|content)-(article|collection)-(transformer|mapper|unfolder)(-pr|-iw)?(-uk-.*)?\\.svc\\.ft\\.com(:\\d{2,5})?/(content)/[\\w-]+.*$" \
        coco/notifications-push
```

Clients
-------

Example client code is provided in `bin/client` directory

### Logging

* The application uses [go-logger](https://github.com/Financial-Times/go-logger ); the log file is initialised in [app.go](app.go).

Useful Links
------------
* Production: 

[https://api.ft.com/content/notifications-push](#https://api.ft.com/content/notifications-push?apiKey=555) (needs API key)

* For internal use:

[https://prod-coco-up-read.ft.com/content/notifications-push](#https://prod-coco-up-read.ft.com/content/notifications-push) (needs credentials) 

[https://prod-coco-up-read.ft.com/lists/notifications-push](#https://prod-coco-up-read.ft.com/lists/notifications-push) (needs credentials)


