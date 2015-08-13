
## idea

I really like UNIX pipes.
The idea that a program processes the input from `stdin` and writes it's results to `stdout`
is so simple, decomposable and interoable.

The problem is you cannot scale it reliable.
A while ago I wrote redis-pipe, a utility that allowed you to stream unix pipes
to a redis or from a redis server and distribute work.

It had one flaw! It lost messages, like alot. In production servers go down,
processes get cancelled and the harddisks lit on fire. You don't want your workload
affected by that.

So I have a new idea.

Let's take the example of redis-pipe.


Now we want to search this pipe with parellel workers for
prenames that start with `Al`.

So we start two workers on different machines.

```
redis-pipe greetings | grep Al
```

I want to be sure that I only work a message once and only once.
And if I fail to produce a result I want to reject it and if it is
successfull I want to ack it.

How to do that.

Specification


Every process that successfully processed a line, writes it to the first passed file descriptor
if it failed to do so it needs to do nothing. If it wants to explicitely reject the message
it needs to write it to the second file descriptor.

Think of it as an extension of `stdin`, `stderr` and `stdout`.
An addition called `stdout-ack` and `stderr-reject`.


Why? Because results can differ from the input.


Sample.

```
redis-pipe greetings <<EOF
Alex
Alice
Bob
EOF
```

And I wrote a Python program that counts occurrences.

```python
import sys
from collections import Counter

counter = Counter([line.strip() for line in sys.stdin])
for line, count in counter.most_common():
    print('{} {}'.format(line ,count))
```

Now if I feed the file

```
Alex
Alice
Bob
Alex
```

It will output

```
Alex 2
Alice 1
Bob 1
```

We now have modified our program to be easily distributable and
in a completly transparent way. If we don't want to use this
method it does no harm because it is piped to `/dev/null` per default.

.By writing each successful operation to `stdack` we have now defined
a protocol how the piper can check whether we have processed
his message (line).

```python
import sys
import os
from collections import Counter

stdack = open(os.getenv('ACK_FH', os.devnull), 'w')

counter = Counter()
for line in sys.stdin:
    stdack.write(line)
    counter.update([line.strip()])

for line, count in counter.most_common():
    sys.stdout.write('{} {}\n'.format(line, count))
```

The piper can do this by using named pipes.

mkfifo ack && \
cat ack | ./redis-pipe greetings | FH_ACK=ack python count.py && \
rm ack

## Contract

Any program that accepts output from `stdin` and writes to `stdout`
should accept an environment variable `FACK` containing a file descriptor.

If a single operation performed on a line from `stdin` was successful ,
that line should be written to `FACK`.

