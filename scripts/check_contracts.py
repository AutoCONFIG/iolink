#!/usr/bin/env python3
"""R02.a: static OpenAPI and synthetic fixture checks (not handler acceptance)."""
from copy import deepcopy
from pathlib import Path
import re
import yaml
from openapi_spec_validator import validate
from openapi_schema_validator import OAS30Validator

ROOT = Path(__file__).resolve().parents[1]

def resolve(value, doc):
    if isinstance(value, dict):
        if '$ref' in value:
            ref = value['$ref']
            assert ref.startswith('#/'), f'external ref not pinned: {ref}'
            item = doc
            for key in ref[2:].split('/'):
                item = item[key.replace('~1', '/').replace('~0', '~')]
            return resolve(item, doc)
        return {key: resolve(item, doc) for key, item in value.items()}
    if isinstance(value, list):
        return [resolve(item, doc) for item in value]
    return value

def fixture(schema):
    s = deepcopy(schema)
    if 'anyOf' in s:
        branch = s['anyOf'][0]
        s.setdefault('properties', {}).update(branch.get('properties', {}))
        s['required'] = list(set(s.get('required', []) + branch.get('required', [])))
    if 'enum' in s:
        return s['enum'][0]
    kind = s.get('type', 'object')
    if kind == 'object':
        value = {key: fixture(s['properties'][key]) for key in s.get('required', [])}
        minimum = s.get('minProperties', 0)
        if len(value) < minimum:
            extra_schema = s.get('additionalProperties')
            if isinstance(extra_schema, dict):
                extra_value = fixture(extra_schema)
            else:
                extra_value = 'x'
            for index in range(minimum - len(value)):
                value[f'fixture_{index}'] = extra_value
        return value
    if kind == 'array':
        return [fixture(s['items']) for _ in range(max(s.get('minItems', 0), 1))]
    if kind in ('integer', 'number'):
        return max(s.get('minimum', 0), 1)
    if kind == 'boolean':
        return True
    if s.get('format') == 'date-time':
        return '2026-09-19T00:00:00Z'
    if s.get('format') == 'byte':
        return 'eA=='
    if s.get('pattern') == '^[0-9a-f]{64}$':
        return 'a' * 64
    return 'x' * max(s.get('minLength', 0), 1)

def main():
    counts = []
    samples = 0
    for path in sorted((ROOT / 'docs/api').glob('*.yaml')):
        doc = yaml.safe_load(path.read_text())
        validate(doc)
        full = resolve(doc, doc)
        count = 0
        for route, methods in full['paths'].items():
            for method, operation in methods.items():
                if method not in {'get', 'post', 'put', 'delete', 'patch'}:
                    continue
                count += 1
                for status, response in operation['responses'].items():
                    if path.name == 'license-openapi.yaml' and int(status) >= 400:
                        assert 'schema' in response.get('content', {}).get('application/json', {}), (route, method, status, 'missing error schema')
                parameters = operation.get('parameters', [])
                assert set(re.findall(r'{([^}]+)}', route)) == {p['name'] for p in parameters if p['in'] == 'path'}
                containers = [operation['requestBody']] if 'requestBody' in operation else []
                containers += list(operation['responses'].values())
                for container in containers:
                    for media in container.get('content', {}).values():
                        schema = media['schema']
                        OAS30Validator(schema, format_checker=OAS30Validator.FORMAT_CHECKER).validate(fixture(schema))
                        samples += 1
        if 'RuleReq' in full['components']['schemas']:
            checker = OAS30Validator(full['components']['schemas']['RuleReq'])
            base = {'pond_id': 1, 'metric': 'ph', 'level': 'warning'}
            assert not checker.is_valid(base)
            assert not checker.is_valid(dict(base, min_value=None, max_value=None))
            assert checker.is_valid(dict(base, min_value=4))
            assert checker.is_valid(dict(base, max_value=9))
        if 'SignedPayload' in full['components']['schemas']:
            signed = full['components']['schemas']['SignedPayload']
            payload = fixture(signed)
            checker = OAS30Validator(signed)
            checker.validate(payload)
            for field in signed['required']:
                missing = {key: value for key, value in payload.items() if key != field}
                assert not checker.is_valid(missing), (path, field, 'missing required signed field accepted')
                samples += 1
            assert not checker.is_valid(dict(payload, features=['video', 'video']))
            assert not checker.is_valid(dict(payload, features=['future']))
            assert not checker.is_valid(dict(payload, max_devices=-1))
            assert not checker.is_valid(dict(payload, unexpected=True))
            samples += 5
        counts.append(count)
        print(f'{path.relative_to(ROOT)}: standard schema and {count} operation fixtures PASS')
    assert sorted(counts) == [3, 5, 15, 44], counts
    print(f'{sum(counts)} target operations, {samples} synthetic request/response fixtures PASS; live handler verification is R02.c')

if __name__ == '__main__':
    main()
