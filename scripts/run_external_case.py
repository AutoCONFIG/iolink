#!/usr/bin/env python3
import argparse
import json
import os
import subprocess

parser = argparse.ArgumentParser()
parser.add_argument('--case', required=True)
parser.add_argument('--provider', required=True)
parser.add_argument('--mode', choices=('software', 'external'), required=True)
args = parser.parse_args()

if args.mode == 'software':
    integration_required = args.case == 'R28-R33'
    integration_available = 'IOLINK_TEST_PG_DSN' in os.environ and bool(os.environ['IOLINK_TEST_PG_DSN'])
    integration_index = None
    if args.case == 'R28-R33':
        commands = [
            ['make', 'verify'],
            ['make', 'verify-contracts'],
            ['make', 'web-verify'],
            ['python3', 'scripts/check_architecture_manifests.py', '--gate', 'M5'],
        ]
        if integration_available:
            integration_index = len(commands)
            commands.append(['make', 'integration'])
    else:
        commands = [['npm', 'test', '--prefix', 'web-mini'], ['npm', 'run', 'typecheck', '--prefix', 'web-mini'], ['npm', 'run', 'build', '--prefix', 'web-mini']]
    results = [subprocess.run(command, check=False, capture_output=True, text=True) for command in commands]
    exit_code = next((result.returncode for result in results if result.returncode != 0), 0)
    software = 'failed' if exit_code != 0 else 'passed'
    if not integration_required:
        integration = 'not_required'
    elif not integration_available:
        integration = 'skipped_external_blocked'
    elif results[integration_index].returncode != 0:
        integration = 'failed'
    else:
        integration = 'passed'
    print(json.dumps({'case': args.case, 'provider': args.provider, 'software': software, 'integration': integration, 'external': 'external_blocked', 'exit_code': exit_code}, ensure_ascii=False))
    raise SystemExit(exit_code)
else:
    print(json.dumps({'case': args.case, 'provider': args.provider, 'external': 'external_blocked', 'reason': 'credentials/device not supplied'}, ensure_ascii=False))
