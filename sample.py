import sys
import os
from collections import Counter

ACK_FH = open(os.getenv('ACK_FH', os.devnull), 'w')

counter = Counter()
for line in sys.stdin:
    ACK_FH.write(line)
    counter.update([line.strip()])

for line, count in counter.most_common():
    sys.stdout.write('{} {}\n'.format(line, count))
