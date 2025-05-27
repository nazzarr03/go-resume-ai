FROM golang:1.23-alpine

WORKDIR /app

COPY . .

RUN go mod download

RUN go build -o go-resume-ai .

EXPOSE 8082

CMD [ "./go-resume-ai" ]