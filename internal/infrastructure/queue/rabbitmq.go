package queue

import (
	"context"
	"encoding/json"
	"github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type RabbitMQ struct{ URL string }

func (q RabbitMQ) EnqueueCrawl(ctx context.Context, id primitive.ObjectID) error {
	conn, e := amqp091.Dial(q.URL)
	if e != nil {
		return e
	}
	defer conn.Close()
	ch, e := conn.Channel()
	if e != nil {
		return e
	}
	defer ch.Close()
	_, e = ch.QueueDeclare("career.crawl", true, false, false, false, amqp091.Table{"x-dead-letter-exchange": "", "x-dead-letter-routing-key": "career.crawl.dead"})
	if e != nil {
		return e
	}
	body, e := json.Marshal(map[string]string{"source_id": id.Hex()})
	if e != nil {
		return e
	}
	return ch.PublishWithContext(ctx, "", "career.crawl", false, false, amqp091.Publishing{DeliveryMode: amqp091.Persistent, ContentType: "application/json", Body: body})
}
