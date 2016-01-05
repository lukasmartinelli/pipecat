# pipecat [![Build Status](https://travis-ci.org/lukasmartinelli/pipecat.svg?branch=master)](https://travis-ci.org/lukasmartinelli/pipecat) ![MIT licensed](https://img.shields.io/badge/license-MIT-blue.svg)

<img align="right" alt="pipecat" src="logo.png" />
Pipecat allows you to scale any program supporting the [FACK contract](#fack-contract)
using traditional UNIX pipes and [AMQP](https://www.amqp.org/).
Think of it as [netcat](http://nc110.sourceforge.net/)
but with message acknowledgments.
It is the successor of [redis-pipe](http://github.com/lukasmartinelli/redis-pipe).

```bash
# Publish sequence of numbers to a job queue.
seq 1 1000 | pipecat publish numbers

# Multiply each number with 10 and store results in a different queue.
pipecat consume numbers --autoack | xargs -n 1 expr 10 '*' | pipecat publish results

# Aggregate the results and calculate the sum
pipecat consume results --autoack --non-blocking \
  | python -cu 'import sys; print(sum(map(int, sys.stdin)))'
```

## Support

Pipecat supports a local mode and all AMQP 0.9.1 message brokers.

- [ActiveMQ](http://activemq.apache.org/)
- [RabbitMQ](https://www.rabbitmq.com/)
- [Azure Service Bus](https://azure.microsoft.com/en-us/services/service-bus/)

## Install

You can download a single binary for Linux, OSX or Windows.

**OSX**

```bash
wget -O pipecat https://github.com/lukasmartinelli/pipecat/releases/download/v0.1/pipecat_darwin_amd64
chmod +x pipecat

./pipecat --help
```

**Linux**

```bash
wget -O pipecat https://github.com/lukasmartinelli/pipecat/releases/download/v0.1/pipecat_linux_amd64
chmod +x pipecat

./pipecat --help
```


**Install from Source**

```
go get github.com/lukasmartinelli/pipecat
```

If you are using Windows or 32-bit architectures you need to [download the appropriate binary
yourself](https://github.com/lukasmartinelli/pipecat/releases/latest).

## Using pipecat

`pipecat` connects message queues and UNIX pipes.
The need arose when I started building messaging support into
utilities in order to make them scalable but still wanted to leave my programs the way they are without heavy dependencies and still be able to scale the process **reliably**.

In this example we will calculate the sum of a sequence of numbers.

### Connect the broker

Specify the `AMQP_URI` env var to connect to the message broker.

```
export AMQP_URI=amqp://user:pass@host:5672/vhost
```

### Create the queue

Let's create a new queue `numbers` and publish a sequence of numbers from 1 to 1000.

```bash
seq 1 1000 | pipecat publish numbers
```

### Process input

Multiply the input sequence with factor `10` and publish the results to an additional `results` queue.
This step can be run on multiple hosts.
We want to acknowledge all received messages automatically with `--autoack`.

```bash
pipecat consume numbers --autoack | xargs -n 1 expr 10 '*' | pipecat publish results
```

### Aggregate results

Now let's sum up all the numbers. Because we want to end after receiving all numbers we specify the `--non-blocking` mode which will close the connection if no messages have been received after a timeout.

```bash
pipecat consume results --autoack --non-blocking | python -cu 'import sys; print(sum(map(int, sys.stdin)))'
```

### Local RabbitMQ with Docker

If you do not have an existing AMQP broker at hand you can run
RabbitMQ in a docker container, expose the ports and connect to it.

```bash
docker run -d -p 5672:5672 --hostname pipecat-rabbit --name pipecat-rabbit rabbitmq:3
```

Now connect to localhost with the default `guest` login.

```bash
export AMQP_URI=amqp://guest:guest@localhost:5672/vhost
```

### Publish messages to Exchange

If you are using existing message queue infrastructure you can also publish messages to an exchange.

```bash
seq 1 1000 | pipecat publish --exchange "\" --no-create-queue numbers
```


## Make it failsafe

We already have written a small, concise and very
scalable set of programs. We can now run the `multiply.py`
step on many servers.

However, if the server dies while `multiply.py` is
running **the input lines already processed are lost**.

If your program needs that ability you need to implement
the [FACK contract](#fack-contract), demonstrated for the `multiply.py` sample.

## FACK Contract

> Any program that accepts output from `stdin` and writes to `stdout`
  should accept an environment variable `FACK` containing a file descriptor.
  If a single operation performed on a line from `stdin` was successful ,
  that line should be written to `FACK`.

![FACK contract Flow](diagrams/fack_contract.png)

### Implement the contract

Implementing the contract is straightforward.

1. Support the optional `FACK` environment variable containing a file name
2. Write the received input into this file handle if we
   performed the operation successfully on it

#### Python Example

Below is a Python example `multiply.py` which multiplies the sequence of numbers as above
but writes the input line to `stdack` if successfully processed.


```python
import sys
import os

with open(os.getenv('FACK', os.devnull), 'w') as stdack: # Works even if FACK is not set
    for line in sys.stdin:
        num = int(line.strip())
        result = num * 10
        sys.stdout.write('{}\n'.format(result))
        stdack.write(line) # Ack the processed line
        stdack.flush() # Make sure line does not get lost in the buffer
```

### Use named queues for ACKs

Now your program can no longer lose messages with `pipecat` because
you can feed the `FACK` output back into `pipecat`
using [named pipes](http://thorstenball.com/blog/2013/08/11/named-pipes/)
which will only then acknowledge the messages from the message queue.

![Pipecat Flow Diagram](diagrams/pipecat_flow.png)

Fill the queue again.

```bash
seq 1 1000 | pipecat publish numbers
```

And use a named pipe to funnel the acknowledged input lines back into
pipecat.

```bash
mkfifo ack
cat ack | pipecat consume numbers \
| FACK=ack python -u multiply.py \
| pipecat publish results
rm ack
```

Consume all messages to reduce a result.
In the reduce operation we need to autoack all received messages
because we can't possibly hold the entire result set in memory until the
operation has performed.

```bash
pipecat consume results --autoack --non-blocking | python -cu 'import sys; print(sum(map(int, sys.stdin)))'
```

With a few lines additional code only depending on the standard library
you can now make any program in any language scalable using message queues.
Without any dependencies and without changing the behavior bit.

## Cross Compile Release

We use [gox](https://github.com/mitchellh/gox) to create distributable
binaries for Windows, OSX and Linux.

```bash
docker run --rm -v "$(pwd)":/usr/src/pipecat -w /usr/src/pipecat tcnksm/gox:1.4.2-light
```
