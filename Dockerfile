FROM alpine:3.3

ADD *.go /notifications-push/

RUN apk add --update bash \
  && apk --update add git bzr \
  && apk --update add go \
  && export GOPATH=/gopath \
  && REPO_PATH="github.com/Financial-Times/notifications-push" \
  && mkdir -p $GOPATH/src/${REPO_PATH} \
  && mv notifications-push/* $GOPATH/src/${REPO_PATH} \
  && cd $GOPATH/src/${REPO_PATH} \
  && go get -t ./... \
  && go build \
  && mv notifications-push /notifications-push-app \
  && rm -rf /notifications-push \
  && mv /notifications-push-app /notifications-push \
  && apk del go git bzr \
  && rm -rf $GOPATH /var/cache/apk/*

CMD [ "/notifications-push" ]