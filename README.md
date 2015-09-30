# pipecat [![Build Status](https://travis-ci.org/lukasmartinelli/pipecat.svg?branch=master)](https://travis-ci.org/lukasmartinelli/pipecat)

<img align="right" alt="pipecat" src="pipecat.png" />
Pipecat allows you to scale any program supporting the [FACK contract](#fack-contract)
using traditional UNIX pipes.
Think of it as [netcat](http://nc110.sourceforge.net/)
but with message acknowledgments.
It is the successor of [redis-pipe](http://github.com/lukasmartinelli/redis-pipe).

## Install

```
go get github.com/lukasmartinelli/pipecat
```

## Support

Pipecat supports a local mode and all AMPQ 0.9.1 message brokers.

- [ActiveMQ](http://activemq.apache.org/)
- [RabbitMQ](https://www.rabbitmq.com/)
- [Azure Service Bus](https://azure.microsoft.com/en-us/services/service-bus/)

## FACK Contract

> Any program that accepts output from `stdin` and writes to `stdout`
  should accept an environment variable `FACK` containing a file descriptor.
  If a single operation performed on a line from `stdin` was successful ,
  that line should be written to `FACK`.

## Using pipecat

`pipecat` provides message queues via UNIX pipes.
The need arose when I started building messaging support into
utilities in order to make them scalable.
I want to leave my programs the way they are without heavy dependencies
and still be able to scale the process.

In this example we will calculate the sum of a sequence of numbers.

### Connect the broker

If you want to use a message broker you need to specify the `AMQP_URI` env var.

```
export AMQP_URI=amqp://user:pass@host:5672/vhost
```

### Create the queue

Let's create a new queue and `publish` a sequence.

```bash
seq 1 1000 | pipecat publish numbers
```

### Multiply numbers

We write a small python program `multiply.py` that
multiplies every number from `stdin`
with 10 and writes the result to `stdout`.

```python
import sys

for line in sys.stdin:
    num = int(line.strip())
    result = num * 10
    sys.stdout.write('{}\n'.format(result))
```

Let's start the `consumer` which reads all numbers from
the `numbers` queue and `publish` the results
in an additional queue.
We want to acknowledge all messages we receive with `--autoack`.

```bash
pipecat consume numbers --autoack | python -u multiply.py | pipecat publish results
```

## Aggregate results

Now we  store the sum of all these numbers
with `sum.py`.

```python
import sys

sum = sum(int(line.strip()) for line in sys.stdin)
sys.stdout.write('{}\n'.format(sum))
```

And now look at the result. Because we want the consumer to eventually
finsish we specify it as `--non-blocking` which will stop consuming if no messages
arrive after a configurable timeout.

```bash
pipecat consume results --autoack --non-blocking | python sum.py
```

## Make it failsafe

We already have written a small, concise and very
scalable set of programs. We can now run the `multiply.py`
step on many servers.

However, if the server dies while `multiply.py` is
running **the input lines already processed are lost**.

If your program needs that ability you need to implement
the [FACK contract](#fack-contract), demonstrated for the `multiply.py` sample.

### Implement the contract

Implementing the contract is straightforward.

1. Support the optional `FACK` environment variable containing a file name
2. Write the received input into this file handle if we
   performed the operation successfully on it

```python
import sys
import os

with open(os.getenv('FACK', os.devnull), 'w') as stdack:
    for line in sys.stdin:
        num = int(line.strip())
        result = num * 10
        sys.stdout.write('{}\n'.format(result))
        stdack.write(line) # Ack the processed line
        stdack.flush() # Make sure line does not get lost in the buffer
```

### Use named queues for ACKs

Now your program can no longer loose messages with `pipecat` because
you can feed the `FACK` output back into `pipecat`
using [named pipes](http://thorstenball.com/blog/2013/08/11/named-pipes/)
which will only then acknowledge the messages from the message queue.

Fill the queue again.

```bash
seq 1 1000 | pipecat publish numbers
```

And use a named pipe to funnel the acknowledged input lines back into
pipecat.

```bash
mkfifo ack
cat ack | pipecat consume numbers
| FACK=ack python -u multiply.py \
| pipecat publish results
rm ack
```

Consume all messages to reduce a result.
In the reduce operation we need to autoack all received messages
because we can't possibly hold the entire result set in memory until the
operation has performed.

```bash
pipecat consume results --autoack --non-blocking | python -u sum.py
```

With a few lines additional code only depending on the standard library
you can now make any program in any language scalable using message queues.
Without any dependencies and without changing the behavior bit.
