import sys
import os

stdack = open(os.getenv('FACK', os.devnull), 'w')

for line in sys.stdin:
    print(line, file=stdack)
    print(line.strip()[::-1])
