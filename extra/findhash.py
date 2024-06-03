import random
import fileinput

keys = [line.strip() for line in fileinput.input()]

M = 0
P = None

for _ in range(10000):
    gs = set()
    for k in keys:
        # use genetic algorithm to find a perfect hash function
        a = random.randint(10, 500)
        s = sum(i * (a + ord(c)) for i, c in enumerate(k))
        g = s % 16384
        gs.add(g)
    # print(len(gs))
    if len(gs) > M:
        M = len(gs)
        P = (a,)
        print(M, P)

print(M)
print(P)

# index = i * (37 + ord(c))
