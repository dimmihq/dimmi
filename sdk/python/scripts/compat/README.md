Deprecated generated names kept after the spec dropped them. generate.sh
copies each `models/<module>.py` here into `_generated/models/` (unless the
spec brings the module back) and exports its class from `models/__init__`.
Remove a shim only in a major release.
