# Suitability known-answer vectors

`cases.json` is the shared input/output corpus for P01. The offline Python reader and
the Go `internal/geoenv` decision engine read the same file. Expected results and
reason codes are written in `scripts/data/build_suitability_cases.py` from the
requirement rules, not generated from either implementation.

Regenerate with `python scripts/data/build_suitability_cases.py`. Tests only read the
committed vectors.
