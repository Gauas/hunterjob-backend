package joblake

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/hunterjob/hunterjob/api/internal/jobevent"
	amqp "github.com/rabbitmq/amqp091-go"
)

const queue = "job.new"

type Publisher struct {
	conn     *amqp.Connection
	channel  *amqp.Channel
	returned <-chan amqp.Return
	stop     func() bool
}

func OpenPublisher(ctx context.Context, brokerURL string) (*Publisher, error) {
	var network net.Conn
	dial := amqp.DefaultDial(10 * time.Second)
	conn, err := amqp.DialConfig(brokerURL, amqp.Config{Dial: func(networkType, address string) (net.Conn, error) {
		var err error
		network, err = dial(networkType, address)
		return network, err
	}})
	if err != nil {
		return nil, fmt.Errorf("connect RabbitMQ: %w", err)
	}
	p := &Publisher{conn: conn, stop: context.AfterFunc(ctx, func() { _ = network.Close() })}
	ch, err := conn.Channel()
	if err != nil {
		p.Close()
		return nil, err
	}
	p.channel = ch
	for _, declaration := range []struct {
		name string
		args amqp.Table
	}{
		{queue + ".dead", nil},
		{queue, amqp.Table{"x-dead-letter-exchange": "", "x-dead-letter-routing-key": queue + ".dead"}},
	} {
		if _, err := ch.QueueDeclare(declaration.name, true, false, false, false, declaration.args); err != nil {
			p.Close()
			return nil, fmt.Errorf("declare %s: %w", declaration.name, err)
		}
	}
	if err := ch.Confirm(false); err != nil {
		p.Close()
		return nil, err
	}
	p.returned = ch.NotifyReturn(make(chan amqp.Return, 1))
	return p, nil
}

func (p *Publisher) Close() {
	defer p.stop()
	_ = p.conn.CloseDeadline(time.Now().Add(5 * time.Second))
}

func (p *Publisher) Publish(ctx context.Context, job jobevent.Job) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	body, err := json.Marshal(job)
	if err != nil {
		return err
	}
	confirmation, err := p.channel.PublishWithDeferredConfirm("", queue, true, false, amqp.Publishing{
		ContentType: "application/json", DeliveryMode: amqp.Persistent, MessageId: job.ID, Body: body,
	})
	if err != nil {
		return err
	}
	wait, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	accepted, err := confirmation.WaitContext(wait)
	if err != nil {
		return err
	}
	if !accepted {
		return errors.New("RabbitMQ rejected the job event")
	}
	select {
	case returned, ok := <-p.returned:
		if ok {
			return fmt.Errorf("RabbitMQ returned job event: %s", returned.ReplyText)
		}
	default:
	}
	return nil
}
