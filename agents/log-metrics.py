#!/usr/bin/env python3
#
# log-metrics.py - Jobster agent for logging metrics to JSONL file
#
# Agent Contract:
# - Receives CONFIG_JSON env var with hook configuration
# - Receives job metadata via env vars (JOB_ID, RUN_ID, HOOK, etc.)
# - Writes metrics to STATE_DIR/metrics.jsonl (one JSON object per line)
# - Outputs JSON status to stdout
# - Exit 0 on success, non-zero on error
#

import os
import sys
import json
from datetime import datetime
from pathlib import Path

def main():
    # Read configuration from environment
    config_json = os.environ.get('CONFIG_JSON', '{}')
    try:
        config = json.loads(config_json)
    except json.JSONDecodeError as e:
        error_output = {
            'status': 'error',
            'error': f'Invalid CONFIG_JSON: {str(e)}'
        }
        print(json.dumps(error_output), file=sys.stderr)
        sys.exit(1)

    # Get STATE_DIR (writable per-job directory)
    state_dir = os.environ.get('STATE_DIR')
    if not state_dir:
        error_output = {
            'status': 'error',
            'error': 'STATE_DIR environment variable not set'
        }
        print(json.dumps(error_output), file=sys.stderr)
        sys.exit(1)

    # Create STATE_DIR if it doesn't exist
    state_path = Path(state_dir)
    state_path.mkdir(parents=True, exist_ok=True)

    # Build metrics object from job metadata
    metrics = {
        'timestamp': datetime.utcnow().isoformat() + 'Z',
        'job_id': os.environ.get('JOB_ID', 'unknown'),
        'run_id': os.environ.get('RUN_ID', 'unknown'),
        'hook': os.environ.get('HOOK', 'unknown'),
        'job_command': os.environ.get('JOB_COMMAND', ''),
        'job_schedule': os.environ.get('JOB_SCHEDULE', ''),
        'start_ts': os.environ.get('START_TS', ''),
        'end_ts': os.environ.get('END_TS', ''),
        'exit_code': int(os.environ.get('EXIT_CODE', '-1')),
        'attempt': int(os.environ.get('ATTEMPT', '1'))
    }

    # Add custom metrics from config
    custom_metrics = config.get('metrics', {})
    if custom_metrics:
        metrics['custom'] = custom_metrics

    # Calculate duration if timestamps available
    if metrics['start_ts'] and metrics['end_ts']:
        try:
            start = datetime.fromisoformat(metrics['start_ts'].replace('Z', '+00:00'))
            end = datetime.fromisoformat(metrics['end_ts'].replace('Z', '+00:00'))
            metrics['duration_sec'] = (end - start).total_seconds()
        except (ValueError, AttributeError):
            pass

    # Append to metrics.jsonl file
    metrics_file = state_path / 'metrics.jsonl'
    try:
        with open(metrics_file, 'a') as f:
            f.write(json.dumps(metrics) + '\n')
    except IOError as e:
        error_output = {
            'status': 'error',
            'error': f'Failed to write metrics: {str(e)}'
        }
        print(json.dumps(error_output), file=sys.stderr)
        sys.exit(1)

    # Output success status
    output = {
        'status': 'ok',
        'metrics': {
            'logged': 1,
            'file': str(metrics_file)
        },
        'notes': f'Metrics appended to {metrics_file}'
    }
    print(json.dumps(output))
    sys.exit(0)

if __name__ == '__main__':
    main()
