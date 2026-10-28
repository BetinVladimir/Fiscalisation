#!/usr/bin/env python3
"""Compatibility entrypoint for the complete MVP1 generation plan."""
import runpy
from pathlib import Path
if __name__=='__main__':
    runpy.run_path(str(Path(__file__).resolve().parent.parent/'fixes/build_plan.py'),run_name='__main__')
