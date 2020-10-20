FROM golang:latest

RUN mkdir /app

ADD . /app/
WORKDIR /app

RUN go get github.com/go-telegram-bot-api/telegram-bot-api
RUN go get github.com/tkanos/gonfig
RUN go get github.com/dustin/go-humanize

RUN go build -o main .

CMD ["/app/main"]