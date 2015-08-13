# ACK for Unix Pipes

I like UNIX pipes. A program process input from `stdin` and writes the results
to `stdout`. Process can be chained together to create powerful workflows.

This is actually cloud ready and an easier concept than heavyweight messaging.

However the big problem is that **you can loose data**.
There is no way to determine whether the program has processed
the input from `stdin` successfully.

I want to solve that with a simple contract a program can implement
and a solution to scale these simple programs to hundreds of servers.

## Using mqpipe

`mqpipe` connects messaging with UNIX pipes. The need arose when I started
building messaging support into every utility in order to make it scalable.
I want to leave my programs the way they are without heavy dependencies
and still be able to scale the process.


`mqpipe` uses a Redis server to implement message queues.
Let's pipe a word list into our message queue.

```
mqpipe wordlist <<EOF
Alex
Alice
Bob
Manfred
EOF
```

We have now created the message queue `wordlist` and filled it with 4 messages.
Let's reverse each name in a fault tolerant way.

```
mkfifo ack && \
FACK=ack mqpipe wordlist | python sample.py | mqpipe results && \
rm ack
```

With this run command and we have fully supported distributing the work
via message queues - by simply implementing the contract explained below.

## Contract

Any program that accepts output from `stdin` and writes to `stdout`
should accept an environment variable `FACK` containing a file descriptor.

If a single operation performed on a line from `stdin` was successful ,
that line should be written to `FACK`.

### Example

I wrote a Python script that reverses each line.

```python
import sys
import os

for line in sys.stdin:
    sys.stdout.write(line.strip()[::-1] + '\n')
```

If we execute the program and the program halts unexpectedly we don't know
what input I we already have processed.

Implementing the contract is really simple.

1. Support the optional `FACK` environment variable
2. Write to `stdack` if we counted a line successfully

```python
import sys
import os

stdack = open(os.getenv('FACK', os.devnull), 'w')

for line in sys.stdin:
    stdack.write(line)
    sys.stdout.write(line.strip()[::-1] + '\n')
```

Now we log each successful operation to `FACK` which enables this program
to be used in conjunction with message queues or in a distributed way.

If we don't want to use acknowledgments we can simply ignore the `FACK` env var
and the program will write the output into the void.
