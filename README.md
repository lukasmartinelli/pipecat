# pipecat

Pipecat allows you to scale any program supporting the [FACK contract]()
using traditional UNIX pipes. Think of it as [netcat](http://nc110.sourceforge.net/)
but with message acknowledgments.

## Support

pipecat supports most message queue mechanisms:

All messaging servers supporting AMPQ 0.9.1:

- [ActiveMQ](http://activemq.apache.org/)
- [RabbitMQ](https://www.rabbitmq.com/)
- [Azure Service Bus](https://azure.microsoft.com/en-us/services/service-bus/)

And others:

- [Redis](http://redis.io/) through [redismq](https://github.com/adjust/redismq)
- [NATS](http://nats.io/)
- [AWS SQS](https://aws.amazon.com/sqs/)

And for demo purposes a local mode using Linux named pipes internally.

## Using pipecat

`pipecat` provides message queues via UNIX pipes.
The need arose when I started building messaging support into every
utility in order to make it scalable.
I want to leave my programs the way they are without heavy dependencies
and still be able to scale the process.

In this example we will calculate the sum a sequence of numbers.

### Create the queue

Let's create a new queue and store the sequence.

```
seq 1 1000 | pipecat numbers
```

### Add two numbers

So we write a small python program `multiply.py` that
multiplies every number from `stdin`
with 10 and writes the result to `stdout`.

```
import sys

for line in sys.stdin:
    num = int(line.strip())
    result = num * 10
    sys.stdout.write('{}\n'.format(result))
```

Let's start the worker and store the results
in an additional queue.


```
pipecat numbers | python multiply.py | pipecat results
```
## Aggregate results

Now we want to store the sum of all these numbers
with `sum.py`.

```python
import sys

sum = sum(int(line.strip() for line in sys.stdin))
sys.stdout.write('{}\n'.format(sum))
```

And now look at the result.

```
pipecat results | python sum.py
```

## Make it failsafe

We already have written a small, concise and very
scalable set of programs. We can now run the `multiply.py`
step on hundreds of servers.

However, if for example the server dies while `multiply.py` is
running. No one know which input lines from `stdin` the program
has already processed.

If your program needs that ability you need to implement
the `FACK` contract , demonstrated at the `multiply.py` sample.

### Contract

Any program that accepts output from `stdin` and writes to `stdout`
should accept an environment variable `FACK` containing a file descriptor.

If a single operation performed on a line from `stdin` was successful ,
that line should be written to `FACK`.

### Implement the contract

Implementing the contract is straightforward in Python.

1. Support the optional `FACK` environment variable containing a file name
2. Write the recevied input into this file handle if we
   performed the operation successfully on it

```
import sys
import os

stdack = open(os.getenv('FACK', sys.devnull), 'w')

for line in sys.stdin:
    num = int(line.strip())
    result = num * 10
    sys.stdout.write('{}\n'.format(result))
    stdack.write(line)
```

### Use named queues for ACKs

Now your program can no longer loose messages with `pipecat` because
you can feed the `FACK` output into `pipecat` which will only then
acknowledge the messages from the message queue.

```
mkfifo ack && \
cat ack | pipecat numbers | FACK=ack python multiply.py | pipecat results && \
rm ack
```

With a few lines additional code only depending on the standard library
you can now make any program in any language scalable using message queues.
Without any dependencies and without changing the behavior a bit.
