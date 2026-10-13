"""Validate local preparation data without executing dotenv or reading references.

No product runtime is provided here. --probe performs a bounded TCP connect and
temporary local binds only; it sends no HTTP, SIP, password, or application data.
"""
import argparse
import ipaddress
import os
from pathlib import Path
import re
import socket
import sys
from urllib.parse import urlsplit

ROOT = Path(__file__).resolve().parents[1]
BIND_KEYS = ('LAB_HTTP_BIND', 'LAB_DEVICE_SIP_BIND',
             'LAB_PLATFORM_SIP_BIND', 'LAB_WEB_DEV_BIND')


class ConfigError(ValueError):
    """Errors contain fixed diagnostics and key names, never supplied values."""


def parse_env(path):
    values = {}
    if not path.exists():
        return values
    try:
        data = path.read_text(encoding='utf-8-sig')
    except (OSError, UnicodeError):
        raise ConfigError('cannot read dotenv as UTF-8') from None
    for number, raw in enumerate(data.splitlines(), 1):
        line = raw.strip()
        if not line or line.startswith('#'):
            continue
        key, separator, value = line.partition('=')
        key, value = key.strip(), value.strip()
        if not separator or not re.fullmatch(r'[A-Z][A-Z0-9_]*', key):
            raise ConfigError(f'invalid dotenv assignment at line {number}')
        if key in values:
            raise ConfigError(f'duplicate dotenv key at line {number}')
        if value.startswith(('"', "'")):
            if len(value) < 2 or value[-1] != value[0]:
                raise ConfigError(f'unclosed quote at line {number}')
            value = value[1:-1]
        if '\x00' in value:
            raise ConfigError(f'invalid dotenv value at line {number}')
        # Deliberately literal: no variable, shell, escape, or inline-comment expansion.
        values[key] = value
    return values


def load_env(root=ROOT, environ=None):
    allowed = set(parse_env(root / '.env.example'))
    values = {}
    for name in ('.env', '.env.local'):
        layer = parse_env(root / name)
        if set(layer) - allowed:
            raise ConfigError('unknown dotenv key; compare .env.example')
        values.update(layer)
    environment = os.environ if environ is None else environ
    values.update({k: v for k, v in environment.items() if k in allowed})
    return values


def required(values, key):
    if not values.get(key):
        raise ConfigError(f'missing {key}')
    return values[key]


def local_host(host, key):
    # No DNS resolution or ambiguous host aliases in this local-only profile.
    try:
        address = ipaddress.ip_address(host)
    except ValueError:
        raise ConfigError(f'{key} requires a literal loopback IPv4 address') from None
    if address.version != 4 or not address.is_loopback:
        raise ConfigError(f'{key} requires a literal loopback IPv4 address')
    return str(address)


def port_value(value, key):
    if not re.fullmatch(r'[0-9]{1,5}', value) or not 1 <= int(value) <= 65535:
        raise ConfigError(f'invalid port in {key}')
    return int(value)


def endpoint(value, key):
    host, separator, port = value.rpartition(':')
    if not separator:
        raise ConfigError(f'invalid endpoint in {key}')
    return local_host(host, key), port_value(port, key)


def validate(values):
    if required(values, 'LAB_PROFILE') != 'local-test':
        raise ConfigError('LAB_PROFILE must be local-test')
    if required(values, 'LAB_SIP_TRANSPORT') not in ('UDP', 'TCP'):
        raise ConfigError('invalid LAB_SIP_TRANSPORT')
    bindings = {k: endpoint(required(values, k), k) for k in BIND_KEYS}
    target_sip = (local_host(required(values, 'TEST_LIVEGBS_SIP_HOST'),
                             'TEST_LIVEGBS_SIP_HOST'),
                  port_value(required(values, 'TEST_LIVEGBS_SIP_PORT'),
                             'TEST_LIVEGBS_SIP_PORT'))
    if not re.fullmatch(r'[0-9]{20}', required(values, 'TEST_LIVEGBS_SIP_ID')):
        raise ConfigError('TEST_LIVEGBS_SIP_ID must contain 20 digits')
    try:
        url = urlsplit(required(values, 'TEST_LIVEGBS_HTTP_URL'))
        if (url.scheme not in ('http', 'https') or url.username is not None
                or url.password is not None or url.path not in ('', '/')
                or url.query or url.fragment):
            raise ValueError()
        target_http = (local_host(url.hostname or '', 'TEST_LIVEGBS_HTTP_URL'),
                       url.port if url.port is not None else (443 if url.scheme == 'https' else 80))
        if not 1 <= target_http[1] <= 65535:
            raise ValueError()
    except ValueError:
        raise ConfigError('invalid local TEST_LIVEGBS_HTTP_URL') from None
    # Reserve separate ports even across TCP/UDP to keep the local topology clear.
    ports = [x[1] for x in bindings.values()] + [target_sip[1], target_http[1]]
    if len(ports) != len(set(ports)):
        raise ConfigError('local and external test ports must be distinct')
    auth = required(values, 'TEST_LIVEGBS_SIP_AUTH_MODE')
    if auth not in ('unknown', 'none', 'digest'):
        raise ConfigError('invalid TEST_LIVEGBS_SIP_AUTH_MODE')
    if auth == 'digest':
        required(values, 'TEST_LIVEGBS_SIP_USERNAME')
        required(values, 'TEST_LIVEGBS_SIP_PASSWORD')
    for key in ('LAB_DATA_DIR', 'LAB_EVIDENCE_DIR'):
        required(values, key)
    return bindings, target_http


def probe(bindings, target_http):
    ok = True
    try:
        with socket.create_connection(target_http, timeout=2):
            pass
        print('passed: external test HTTP port accepts TCP (HTTP/login/SIP unverified)')
    except OSError:
        print('failed: external test HTTP TCP port is unreachable from this process')
        ok = False
    for key, address in bindings.items():
        kinds = (socket.SOCK_DGRAM, socket.SOCK_STREAM) if 'SIP' in key else (socket.SOCK_STREAM,)
        for kind in kinds:
            label = 'UDP' if kind == socket.SOCK_DGRAM else 'TCP'
            try:
                with socket.socket(socket.AF_INET, kind) as sock:
                    if hasattr(socket, 'SO_EXCLUSIVEADDRUSE'):
                        sock.setsockopt(socket.SOL_SOCKET, socket.SO_EXCLUSIVEADDRUSE, 1)
                    sock.bind(address)
                print(f'passed: {key} {label} temporarily bindable; released')
            except OSError:
                print(f'failed: {key} {label} cannot bind; existing processes untouched')
                ok = False
    return ok


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--probe', action='store_true', help='TCP connect and local bind checks only')
    args = parser.parse_args(argv)
    try:
        values = load_env()
        bindings, target_http = validate(values)
    except ConfigError as error:
        print(f'failed: {error}', file=sys.stderr)
        return 2
    print('passed: local preparation configuration; values and credentials withheld')
    for key in ('TEST_LIVEGBS_USERNAME', 'TEST_LIVEGBS_PASSWORD'):
        print(f'{key}: ' + ('configured' if values.get(key) else 'not configured'))
    if values['TEST_LIVEGBS_SIP_AUTH_MODE'] == 'unknown':
        print('not-run: SIP authentication discovery; Web password is not a SIP password')
    for key in ('GB28181_STANDARDS_ROOT', 'GB28181_REFERENCE_ROOT', 'TEST_LIVEGBS_SOURCE_ROOT'):
        value = values.get(key, '')
        try:
            present = bool(value) and (ROOT / value).is_dir()
        except OSError:
            present = False
        print(f'{key}: ' + ('directory exists (contents not read)' if present else 'not available'))
    return (0 if probe(bindings, target_http) else 1) if args.probe else 0


if __name__ == '__main__':
    sys.exit(main())
