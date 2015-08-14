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
		cli.StringFlag{
			Name:   "fack",
			Value:  os.DevNull,
			Usage:  "ACK file",
			EnvVar: "FACK",
		},
	}

	app.Commands = []cli.Command{
		{
			Name:    "publish",
			Aliases: []string{"p"},
			Usage:   "Publish messages to queue",
			Flags:   globalFlags,
			Action: func(c *cli.Context) {
				queueName := c.Args().First()
				if queueName == "" {
					fmt.Println("Please provide name of the queue")
					os.Exit(1)
				}

				conn, err := amqp.Dial(c.String("amqpUri"))
				failOnError(err, "Failed to connect to AMPQ broker")
				defer conn.Close()

				channel, err := conn.Channel()
				failOnError(err, "Failed to open a channel")
				defer channel.Close()

				q, err := channel.QueueDeclare(
					queueName, // name
					true,      // durable
					false,     // delete when unused
					false,     // exclusive
					false,     // no-wait
					nil,       // arguments
				)
				failOnError(err, "Failed to declare a queue")

				stdack, err := os.OpenFile(c.String("fack"), os.O_APPEND, 0660)
				failOnError(err, "Could not open ack file")
				defer stdack.Close()

				scanner := bufio.NewScanner(os.Stdin)
				for scanner.Scan() {
					line := scanner.Text()
					err = channel.Publish(
						"",     // exchange
						q.Name, // routing key
						false,  // mandatory
						false,  // immediate
						amqp.Publishing{
							ContentType: "text/plain",
							Body:        []byte(line),
						})

					failOnError(err, "Failed to publish a message")
					fmt.Println(line)
					fmt.Fprintln(stdack, line)
				}
				if err := scanner.Err(); err != nil {
					fmt.Fprintln(os.Stderr, "Reading standard input:", err)
				}

			},
		},
		{
			Name:    "consume",
			Flags:   globalFlags,
			Aliases: []string{"c"},
			Usage:   "Consume messages from queue",
			Action: func(c *cli.Context) {
				queueName := c.Args().First()
				if queueName == "" {
					fmt.Println("Please provide name of the queue")
					os.Exit(1)
				}

				conn, err := amqp.Dial(c.String("amqpUri"))
				failOnError(err, "Failed to connect to AMPQ broker")
				defer conn.Close()

				channel, err := conn.Channel()
				failOnError(err, "Failed to open a channel")
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

				stdack, err := os.OpenFile(c.String("fack"), os.O_APPEND, 0660)
				failOnError(err, "Could not open ack file")
				defer stdack.Close()
				forever := make(chan bool)

				go func() {

					scanner := bufio.NewScanner(os.Stdin)
					for scanner.Scan() {
						ackedLine := scanner.Text()

						mutex.Lock()
						for _, msg := range unackedMessages {
							unackedLine := fmt.Sprintf("%s", msg.Body)
							if unackedLine == ackedLine {
								fmt.Fprintln(stdack, ackedLine)
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

			},
		},
	}

	app.Run(os.Args)
}
