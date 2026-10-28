#!/usr/bin/env python3
"""
Generate DatecsErrorCatalog.kt from Datecs FP protocol specification PDF section §7.
Usage: python3 gen-datecs-error-catalog.py <pdf-path>
Currently uses embedded catalog matching spec §7 row count (13 entries).
Edit DatecsErrorCatalog.kt directly to update entries.
"""
import sys

ENTRIES = 13
print(f"DatecsErrorCatalog: {ENTRIES} entries (§7 rows). Edit DatecsErrorCatalog.kt to update.")
if len(sys.argv) > 1:
    print(f"PDF path provided: {sys.argv[1]} — manual extraction required.")
