notifications-push
==================

[![CircleCI](https://circleci.com/gh/Financial-Times/notifications-push.svg?style=svg)](https://circleci.com/gh/Financial-Times/notifications-push) [![Go Report Card](https://goreportcard.com/badge/github.com/Financial-Times/notifications-push)](https://goreportcard.com/report/github.com/Financial-Times/notifications-push) [![Coverage Status](https://coveralls.io/repos/github/Financial-Times/notifications-push/badge.svg)](https://coveralls.io/github/Financial-Times/notifications-push)

Notifications-push is a microservice that provides push notifications of publications or changes of articles and lists of content.
The microservice consumes a specific Apache Kafka topic group, then it pushes a notification for each article or list available in the consumed Kafka messages.

How to Build & Run the binary
-----------------------------

1. Install, build and test:
```
go get github.com/Financial-Times/notifications-push
cd $GOPATH/src/github.com/Financial-Times/notifications-push

go test ./... -race
go install
```
2. Run locally:

* Create tunnel to the Kafka service inside the cluster for ports 2181 and 9092 (use the public IP):
```
kubectl port-forward service/kafka-zookeeper 2181
kubectl port-forward kafka-0 9092
```

* Add the private DNS of the Kafka machine to the hosts file:
```
127.0.0.1       <private_dns> 
```

* Start the service using environment variables:

  For content push notifications:
```
export NOTIFICATIONS_RESOURCE=content \
    && export KAFKA_ADDRS=localhost:2181 \
    && export GROUP_ID=notifications-push-yourtest \
    && export TOPIC=PostPublicationEvents \
    && export NOTIFICATIONS_DELAY=10 \
    && export API_BASE_URL="http://api.ft.com" \
    && export CONTENT_TYPE_WHITELIST="application/vnd.ft-upp-article+json,application/vnd.ft-upp-content-package+json" \
    && export CONTENT_URI_WHITELIST="^http://(methode|wordpress|content)-(article|collection|content-placeholder)-(transformer|mapper|unfolder)(-pr|-iw)?(-uk-.*)?\\.svc\\.ft\\.com(:\\d{2,5})?/(content)/[\\w-]+.*$" \
    && export ALLOWED_ALL_CONTENT_TYPE="Article,ContentPackage,Audio" \
    && export SUPPORTED_SUBSCRIPTION_TYPE="Annotations,Article,ContentPackage,Audio,All,LiveBlogPackage,LiveBlogPost,Content"
    && export DEFAULT_SUBSCRIPTION_TYPE="Article"
    && ./notifications-push
``` 
or for list push notifications:   
```
export NOTIFICATIONS_RESOURCE=lists \
    && export KAFKA_ADDRS=localhost:2181 \
    && export GROUP_ID=notifications-push-yourtest \
    && export TOPIC=PostPublicationEvents \
    && export NOTIFICATIONS_DELAY=10 \
    && export API_BASE_URL="http://api.ft.com" \
    && export CONTENT_TYPE_WHITELIST="application/vnd.ft-upp-list+json" \
    && export DEFAULT_SUBSCRIPTION_TYPE="List"
    && ./notifications-push
```

or for pages push notifications: 
```
export NOTIFICATIONS_RESOURCE=pages \
    && export KAFKA_ADDRS=localhost:2181 \
    && export GROUP_ID=notifications-push-yourtest \
    && export TOPIC=PostPublicationEvents \
    && export NOTIFICATIONS_DELAY=10 \
    && export API_BASE_URL="http://api.ft.com" \
    && export CONTENT_TYPE_WHITELIST="application/vnd.ft-upp-page+json" \
    && export DEFAULT_SUBSCRIPTION_TYPE="Page"
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
    --api_base_url="http://api.ft.com" \
    --api_key_validation_endpoint="t800/a" \
    --content_type_whitelist="application/vnd.ft-upp-article+json" --content_type_whitelist="application/vnd.ft-upp-content-package+json" \
    --content_uri_whitelist="^http://(methode|wordpress|content)-(article|collection|content-placeholder)-(transformer|mapper|unfolder)(-pr|-iw)?(-uk-.*)?\\.svc\\.ft\\.com(:\\d{2,5})?/(content)/[\\w-]+.*$" \
    --allowed_all_contentType="Article,ContentPackage,Audio" \
    --supported_subscription_type="Annotations,Article,ContentPackage,Audio,All,LiveBlogPackage,LiveBlogPost,Content"
    --default_subscription_type="Article"
```

NB: for the complete list of options run `./notifications-push -h`

HTTP endpoints
----------
### For content:  
```curl -i --header "x-api-key: «api_key»" https://api.ft.com/content/notifications-push```

API Keys that allow subscription to content push notifications are `Push API - Content` and `Push API - Internal`.

The following subscription types could be also specified for which the client would like to receive notifications by setting a "type" parameter on the request:

* `Article`
* `ContentPackage`
* `Content` a.k.a ContentPlaceholder
* `Audio`
* `LiveBlogPackage`
* `LiveBlogPost`
* `All`- all content changes (Article, ContentPackage, Audio, Content), but not Annotation, LiveBlogPackage and LiveBlogPost changes.
* `Annotations` - notifications for manual annotation changes

If not specified, by default `Article` is used. If an invalid type is requested an HTTP 400 Bad Request is returned.

If a policy called `ADVANCED_NOTIFICATIONS` is attached to the API key, then the subscriber will be able to
distinguish between content creation and updates for the Article, LiveBlogPackage, LiveBlogPost and Content types.

E.g.
```curl -i --header "x-api-key: «api_key»" https://api.ft.com/content/notifications-push?type=Article```

You can be subscribed for multiple types:
```curl -i --header "x-api-key: «api_key»" https://api.ft.com/content/notifications-push?type=All&type=LiveBlogPost&type=LiveBlogPackage```

### For lists:   
API Key that allows subscription to list push notifications is `Push API - Internal`.

```curl -i --header "x-api-key: «api_key»" https://api.ft.com/lists/notifications-push```

### For pages:
API Key that allows subscription to page push notifications is `Push API - Internal`.

```curl -i --header "x-api-key: «api_key»" https://api.ft.com/pages/notifications-push```

#### Note: 
The lists and pages endpoints only support push notifications of their type (`List` and `Page` respectively), so no `?type=` parameter is supported.
Lists and pages also do not have metadata, and they don't support `Annotations` or other metadata subscription types.

### Filter DELETE messages by type
When a content has been deleted (`http://www.ft.com/thing/ThingChangeType/DELETE`), the kafka payload is empty and we cannot extract the content type from the message. In this case, there are 2 possible behaviours:

* if the `Content-Type` header is empty or equal to `application-json`, then we cannot resolve the content type of the message. Therefore, the notification will be sent to all the subscribers despite their accepted content type.
* if the `Content-Type` header has been specified and it is not equal to `application-json`, then we can resolve the content type based on this header.

e.g. a DELETE message with header `Content-Type: application/vnd.ft-upp-audio+json` will be considered an Audio content. Therefore, the notification will be sent to those subscribers who accept `type=Audio` or `type=All`, but not `type=Annotations`.

### Content Push stream

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

### Annotations Push Stream

```
curl -i --header "x-api-key: «api_key»" https://api.ft.com/content/notifications-push?type=Annotations
```

By opening a HTTP connection with a GET method to the `/content/notifications-push?type=Annotations` endpoint, subscribers can consume the notifications push stream for annotation changes. The stream will look similar to the content one.

#### Message format

```
{
  "apiUrl":"http://api.ft.com/content/4de8b414-c5aa-11e9-a8e9-296ca66511c9",
  "id":"http://www.ft.com/thing/4de8b414-c5aa-11e9-a8e9-296ca66511c9",
  "type":"http://www.ft.com/thing/ThingChangeType/ANNOTATIONS_UPDATE"
}
```

### Monitoring

The notifications-push stream endpoint allows a `monitor` query parameter. By setting the `monitor` flag as `true`, the push stream returns `publishReference` and `lastModified` attributes in the notification message, which is necessary information for UPP internal monitors such as [PAM](https://github.com/Financial-Times/publish-availability-monitor).

There is a special kind of synthetic e2e test publishes that are used for capability monitoring and notifications for them are send only to monitoring subscribers e.g. PAM. The way these kind of publishes are distinguished from any other is by their transaction id which should contain a content UUID that is contained in a configured allowlist.

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
An HTTP GET to the `/__history` endpoint will return the history of the last notifications consumed from the Kafka queue.
In order to access this endpoint port-forwarding must be used:

```shell
kubectl port-forward <pod-name> 8080:8080
```

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
An HTTP GET to the `/__stats` endpoint will return the stats about the current subscribers that are consuming the notifications push stream.
In order to access this endpoint port-forwarding must be used:

```shell
kubectl port-forward <pod-name> 8080:8080
```

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
        --env ALLOWED_ALL_CONTENT_TYPE="Article,ContentPackage,Audio" \
        --env SUPPORTED_SUBSCRIPTION_TYPE="Annotations,Article,ContentPackage,Audio,All,LiveBlogPackage,LiveBlogPost,Content" \
        --env DEFAULT_SUBSCRIPTION_TYPE="Article" \
        coco/notifications-push
```


Running locally with docker-compose
------------------------------

```
docker build -t local/notifications-push:latest .
docker-compose up -d
```

!Note: If the app crashes it is probably kafka is not ready yet, give it a minute and then execute:
```
docker-compose up -d app
```

Install [kafkacat](https://github.com/edenhill/kafkacat) and configure it like this:
```
printf "api.version.request=false\nbroker.version.fallback=0.8.2.0" > ~/.config/kafkacat.conf
```

Prepare a file like this one `payload.ftmessage` containing similar payload:
```
FTMSG/1.0
Content-Type: application/vnd.ft-upp-live-blog-post+json

{
    "payload": {
        "brands": [
            {
                "id": "http://api.ft.com/things/5c7592a8-1f0c-11e4-b0cb-b2227cce2b54"
            }
        ],
        "title": "LiveBlogPostTitle",
        "uuid": "661516b6-2917-42fb-92b9-326bb205ae53",
        "type": "LiveBlogPost",
        "bodyXML": "<body />"
    },
    "lastModified": "2020-08-11T09:00:00.020Z",
    "contentURI": "http://methode-article-internal-components-mapper.svc.ft.com/internalcomponents/661516b6-2917-42fb-92b9-326bb205ae53"
}
```

and send it to kafka like this:
```
kafkacat -P -b localhost:9092 -t PostPublicationEvents -p 0 payload.ftmessage
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


