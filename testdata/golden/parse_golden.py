"""Write what gt-telem 1.2.1 parses from the golden A and B datagrams, for parse_test.go.

Run once, by hand, never in CI: with Python and gt-telem 1.2.1 (the version v1,
those-who-run/those-who-run, pins), from the repository's root:

    python3 testdata/golden/parse_golden.py

For a.datagram and b.datagram (make_golden.py's), it runs gt-telem's own receive
path, TurismoClient._handle_data (decrypt, magic check, _parse_telemetry), with the
heartbeat of the datagram's kind, and writes every field of the Telemetry it makes
to a.gt-telem.json and b.gt-telem.json. The client is built without its constructor,
which looks for a PlayStation on the network. Floats are written as Python's repr
of the double gt-telem holds ("nan" and "inf" included, which JSON numbers can't
carry); integers as they are.

"~" is left out: gt-telem 1.2.1 reads its block straight after car_code, skipping
the B block, so its "~" values are not an oracle (those-who-run/paddock#31).
"""

import dataclasses
import json
import logging
import pathlib
import threading

from gt_telem.net.crypto import PDEncyption
from gt_telem.trackdetector.track_detector import TrackDetector
from gt_telem.turismo_client import TurismoClient

HERE = pathlib.Path(__file__).parent


def client(heartbeat: str) -> TurismoClient:
    c = TurismoClient.__new__(TurismoClient)
    c.heartbeat_type = heartbeat
    c.logger = logging.getLogger("parse_golden")
    c._crypto = PDEncyption(True, heartbeat)
    c._telem_lock = threading.Lock()
    c._telem = None
    c._telem_update_callbacks = {}
    c._track_detector = TrackDetector()
    return c


def value(v):
    if isinstance(v, float):
        return repr(v)
    return v


def main() -> None:
    for name, heartbeat in (("a", "A"), ("b", "B")):
        c = client(heartbeat)
        c._handle_data((HERE / f"{name}.datagram").read_bytes())
        tel = c.telemetry
        assert tel is not None, f"gt-telem did not parse {name}.datagram"
        out = {f.name: value(getattr(tel, f.name)) for f in dataclasses.fields(tel)}
        (HERE / f"{name}.gt-telem.json").write_text(json.dumps(out, indent=1, sort_keys=True) + "\n")


if __name__ == "__main__":
    main()
