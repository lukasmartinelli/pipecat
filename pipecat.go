package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/codegangsta/cli"
	"github.com/streadway/amqp"
)

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
		panic(fmt.Sprintf("%s: %s", msg, err))
	}
}

func prepare(amqpUri string, queueName string) (*amqp.Connection, *amqp.Channel) {
	conn, err := amqp.Dial(amqpUri)
	failOnError(err, "Failed to connect to AMPQ broker")

	channel, err := conn.Channel()
	failOnError(err, "Failed to open a channel")

	_, err = channel.QueueDeclare(
		queueName, // name
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	failOnError(err, "Failed to declare queue")

	return conn, channel
}

func publish(c *cli.Context) {
	queueName := c.Args().First()
	if queueName == "" {
		fmt.Println("Please provide name of the queue")
		os.Exit(1)
	}

	conn, channel := prepare(c.String("amqpUri"), queueName)
	defer conn.Close()
	defer channel.Close()

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		err := channel.Publish(
			"",        // exchange
			queueName, // routing key
			false,     // mandatory
			false,     // immediate
			amqp.Publishing{
				ContentType: "text/plain",
				Body:        []byte(line),
			})

		failOnError(err, "Failed to publish a message")
		fmt.Println(line)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "Reading standard input:", err)
	}
}

func consume(c *cli.Context) {
	queueName := c.Args().First()
	if queueName == "" {
		fmt.Println("Please provide name of the queue")
		os.Exit(1)
	}

	conn, channel := prepare(c.String("amqpUri"), queueName)
	defer conn.Close()
	defer channel.Close()

	var mutex sync.Mutex
	unackedMessages := make([]amqp.Delivery, 100)

	msgs, err := channel.Consume(
		queueName, // queue
		"",        // consumer
		false,     // auto-ack
		false,     // exclusive
		false,     // no-local
		false,     // no-wait
		nil,       // args
	)
	failOnError(err, "Failed to register a consumer")

	forever := make(chan bool)

	go func() {

		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			ackedLine := scanner.Text()

			mutex.Lock()

			// O(n²) complexity for the win!
			for _, msg := range unackedMessages {
				unackedLine := fmt.Sprintf("%s", msg.Body)
				if unackedLine == ackedLine {
					msg.Ack(false)
				}
			}
			mutex.Unlock()
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "Reading standard input:", err)
		}

	}()

	go func() {
		for msg := range msgs {
			mutex.Lock()
			unackedMessages = append(unackedMessages, msg)
			mutex.Unlock()

			line := fmt.Sprintf("%s", msg.Body)
			fmt.Println(line)
		}
	}()
	<-forever
}

func main() {
	app := cli.NewApp()
	app.Name = "pipecat"
	app.Usage = "Connect unix pipes and message queues"

	globalFlags := []cli.Flag{
		cli.StringFlag{
			Name:   "amqpUri",
			Value:  "amqp://guest:guest@localhost:5672/",
			Usage:  "AMQP URI",
			EnvVar: "AMQP_URI",
		},
	}

	app.Commands = []cli.Command{
		{
			Name:    "publish",
			Aliases: []string{"p"},
			Usage:   "Publish messages to queue",
			Flags:   globalFlags,
			Action:  publish,
		},
		{
			Name:    "consume",
			Flags:   globalFlags,
			Aliases: []string{"c"},
			Usage:   "Consume messages from queue",
			Action:  consume,
		},
	}

	app.Run(os.Args)
}
