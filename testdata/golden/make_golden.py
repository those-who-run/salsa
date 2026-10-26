"""Make the golden datagrams for decrypt_test.go, checked by an oracle that is not salsa.

Run once, by hand, never in CI: with Python, pycryptodome and gt-telem 1.2.1 (the
versions v1, those-who-run/those-who-run, pins), from the repository's root:

    python3 testdata/golden/make_golden.py

For each kind (A, B and "~") it builds a synthetic plaintext (the GT7 magic, then a
byte pattern), encrypts it with pycryptodome's Salsa20 under GT7's key and the kind's
nonce, and writes the seed at 0x40 of the datagram as the PS5 does. gt-telem's own
PDEncyption.decrypt then decrypts the datagram, and its output is the committed
plaintext. The script fails unless that output matches the plaintext it started from
(bar the seed's four bytes, which decrypt to the seed XOR the keystream).

These are synthetic, not captured from a PS5: they show salsa agrees with gt-telem,
not with the game. The known-answer test over real captures is
those-who-run/paddock#30's R8.

Files, per kind K (a, b, tilde): K.datagram (as received) and K.plain (gt-telem's
decryption). SHA256SUMS lists them.
"""

import hashlib
import pathlib
import struct

from Crypto.Cipher import Salsa20
from gt_telem.net.crypto import PDEncyption

KEY = b"Simulator Interface Packet GT7 ver 0.0"[:32]
MAGIC = 0x47375330
SEED_AT = 0x40
# name, heartbeat, length, IV mask, seed
KINDS = [
    ("a", "A", 0x128, 0xDEADBEAF, 0x01234567),
    ("b", "B", 0x13C, 0xDEADBEEF, 0x89ABCDEF),
    ("tilde", "~", 0x158, 0x55FABB4F, 0x0BADF00D),
]
HERE = pathlib.Path(__file__).parent


def main() -> None:
    sums = []
    for name, heartbeat, length, mask, seed in KINDS:
        plain = bytearray(struct.pack("<I", MAGIC))
        plain += bytes((i * 31 + length) & 0xFF for i in range(4, length))
        nonce = struct.pack("<II", seed ^ mask, seed)
        datagram = bytearray(Salsa20.new(key=KEY, nonce=nonce).encrypt(bytes(plain)))
        datagram[SEED_AT : SEED_AT + 4] = struct.pack("<I", seed)
        oracle = PDEncyption(True, heartbeat).decrypt(bytes(datagram))
        assert len(oracle) == length
        assert oracle[:SEED_AT] == plain[:SEED_AT], name
        assert oracle[SEED_AT + 4 :] == plain[SEED_AT + 4 :], name
        assert struct.unpack("<I", oracle[:4])[0] == MAGIC, name
        for suffix, data in (("datagram", bytes(datagram)), ("plain", oracle)):
            path = HERE / f"{name}.{suffix}"
            path.write_bytes(data)
            sums.append(f"{hashlib.sha256(data).hexdigest()}  {path.name}\n")
    (HERE / "SHA256SUMS").write_text("".join(sums))


if __name__ == "__main__":
    main()
