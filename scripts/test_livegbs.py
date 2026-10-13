"""Opt-in interoperability runner; never loaded by the product or run by CI.

One synthetic device/channel, zero media, normal heartbeat clocks, <10 min/run.
Inputs, API replies and original wire evidence remain in ignored artifacts.
"""
import argparse
import hashlib
import json
import os
import re
import secrets
import subprocess
import sys
import time
import urllib.parse
import urllib.request

from interop_evidence import inspect_evidence
from local_env import ROOT, ConfigError, load_env, validate


def resolve_binding(values, base, serial):
    """An explicit empty password never falls back to another credential."""
    platform, realm, port = (str(base.get(k, '')) for k in ('Serial', 'Realm', 'Port'))
    if not re.fullmatch(r'[0-9]{20}', platform) or not realm:
        raise ConfigError('verified SIP identity and realm unavailable')
    if port != values['TEST_LIVEGBS_SIP_PORT'] or platform != values['TEST_LIVEGBS_SIP_ID']:
        raise ConfigError('discovered SIP target differs from authorized target')
    if values.get('TEST_LIVEGBS_SIP_REALM') and values['TEST_LIVEGBS_SIP_REALM'] != realm:
        raise ConfigError('configured and discovered SIP realms differ')
    if values.get('TEST_LIVEGBS_SIP_AUTH_MODE') not in ('unknown', 'digest'):
        raise ConfigError('this test requires Digest authentication')
    password = values.get('TEST_LIVEGBS_SIP_PASSWORD', base.get('DevicePassword', ''))
    if not isinstance(password, str) or not password or set(password) <= set('*•'):
        raise ConfigError('verified SIP password unavailable')
    return {'platformId': platform, 'domain': realm,
            'target': values['TEST_LIVEGBS_SIP_HOST'] + ':' + port,
            'username': values.get('TEST_LIVEGBS_SIP_USERNAME') or serial,
            'password': password, 'authMode': 'digest'}


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *args, **kwargs):
        raise ConfigError('test API redirects are not allowed')


class PlatformClient:
    def __init__(self, values):
        self.url = values['TEST_LIVEGBS_HTTP_URL'].rstrip('/')
        self.token = ''
        self.deadline = time.monotonic() + 570
        self.opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())

    def __call__(self, path, params=None, post=False):
        remaining = self.deadline - time.monotonic()
        if remaining <= 0:
            raise TimeoutError('test budget exhausted')
        data = dict(params or {})
        if self.token:
            data['token'] = self.token
        encoded = urllib.parse.urlencode(data).encode()
        request = urllib.request.Request(
            self.url + path + ('' if post else '?' + encoded.decode()),
            data=encoded if post else None)
        with self.opener.open(request, timeout=min(20, remaining)) as response:
            raw = response.read(4 * 1024 * 1024 + 1)
        if len(raw) > 4 * 1024 * 1024:
            raise ConfigError('test API response exceeds budget')
        result = json.loads(raw) if raw.strip() else {}
        if path in ('/api/v1/logout', '/api/v1/device/remove') and result == 'OK':
            return {}
        if not isinstance(result, dict):
            raise ConfigError('unexpected test API response shape')
        return result


def private_json(path, data):
    path.write_text(json.dumps(data, ensure_ascii=False, indent=2), encoding='utf-8')


def source_fingerprint():
    digest = hashlib.sha256()
    files = sorted([*ROOT.glob('internal/**/*.go'), *ROOT.glob('scripts/*.py'),
                    ROOT / 'go.mod', ROOT / 'go.sum'])
    for path in files:
        digest.update(path.relative_to(ROOT).as_posix().encode() + b'\0' + path.read_bytes())
    return digest.hexdigest()


def protect_directory(directory):
    directory.mkdir(parents=True, mode=0o700)
    if os.name == 'nt':
        # Protect the new directory before writing any private input.
        identity = subprocess.check_output(
            ['whoami', '/user', '/fo', 'csv', '/nh'], text=True, encoding='utf-8')
        sid = re.search(r'S-1-[0-9-]+', identity).group(0)
        subprocess.run(['icacls', str(directory), '/inheritance:r', '/grant:r',
                        '*' + sid + ':(OI)(CI)F', '*S-1-5-18:(OI)(CI)F',
                        '*S-1-5-32-544:(OI)(CI)F'], check=True,
                       stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    else:
        directory.chmod(0o700)


def owned_devices(call, serial):
    result = call('/api/v1/device/list', {'q': serial, 'limit': '2'})
    if not isinstance(result.get('DeviceList'), list):
        raise ConfigError('device list unavailable; absence is unconfirmed')
    return [d for d in result['DeviceList'] if d.get('ID') == serial]


def cleanup_owned(call, serial):
    """Caller must have proved this exact identity absent before sending SIP."""
    devices = owned_devices(call, serial)
    if not devices:
        return 'absent'
    if len(devices) != 1 or devices[0].get('Online') is not False:
        return 'unconfirmed: test device is online or its state is unknown'
    call('/api/v1/device/remove', {'serial': serial}, True)
    return 'removed' if not owned_devices(call, serial) else 'unconfirmed: removal not observed'


def run_session(input_data, directory, on_ready):
    """Execute a directly owned test binary so cancellation cannot orphan go test."""
    private = directory / 'input.private.json'
    private_json(private, input_data)
    binary = directory / ('interop.test.exe' if os.name == 'nt' else 'interop.test')
    env = {k: v for k, v in os.environ.items()
           if not k.startswith('TEST_') and k != 'LAB_M0_EXTERNAL_TEST'}
    started = time.monotonic()
    with (directory / 'runner.log').open('w', encoding='utf-8') as log:
        subprocess.run(['go', 'test', '-c', '-o', str(binary), './internal/app'],
                       cwd=ROOT, env=env, stdout=log, stderr=subprocess.STDOUT,
                       check=True, timeout=90)
        env['LAB_M0_EXTERNAL_TEST'] = str(private)
        process = subprocess.Popen([str(binary), '-test.run=^TestControlledExternalInterop$',
                                    '-test.v', '-test.timeout=240s'], cwd=ROOT, env=env,
                                   stdout=log, stderr=subprocess.STDOUT)
        try:
            ready_deadline = time.monotonic() + 30
            while process.poll() is None and not (directory / 'ready').exists():
                if time.monotonic() >= ready_deadline:
                    raise TimeoutError('registration readiness deadline exceeded')
                time.sleep(0.2)
            if (directory / 'ready').exists():
                print('Registered one isolated device; observing queries and three normal heartbeat cycles.', flush=True)
                on_ready()
            result = process.wait(timeout=max(1, 340 - (time.monotonic() - started)))
            return {'exitCode': result, 'elapsedSeconds': round(time.monotonic() - started, 1)}
        finally:
            if process.poll() is None:
                (directory / 'stop').write_text('stop', encoding='ascii')
                try:
                    process.wait(timeout=20)
                except subprocess.TimeoutExpired:
                    process.terminate()
                    process.wait(timeout=10)


def execute_run(values, transport, directory, call, session=run_session, mode='standard'):
    summary = {'transport': transport, 'mediaSessions': 0, 'deviceCount': 1, 'channelCount': 1,
               'protocolMode': mode, 'sourceFingerprint': source_fingerprint(),
               'cleanup': 'not-run', 'wire': {'passed': False}}
    serial = '3402000000118' + str(secrets.randbelow(9000000) + 1000000)
    channel = serial[:10] + '131' + serial[13:]
    owned = False
    try:
        if mode not in ('standard', 'compatibility'):
            raise ConfigError('unknown protocol mode')
        base = call('/api/v1/getbaseconfig')
        binding = resolve_binding(values, base, serial)
        if owned_devices(call, serial):
            raise ConfigError('test identity already exists')
        owned = True
        input_data = {'Binding': binding, 'DeviceID': serial, 'ChannelID': channel,
                      'Transport': transport, 'ProtocolMode': mode, 'EvidenceDir': str(directory)}
        private_json(directory / 'base.private.json', base)

        def on_ready():
            for name, path, params in (
                    ('catalog', '/api/v1/device/fetchcatalog', {'serial': serial, 'timeout': '8'}),
                    ('deviceinfo', '/api/v1/device/fetchinfo',
                     {'serial': serial, 'code': serial, 'timeout': '8', 'update': 'false'})):
                try:
                    result = call(path, params)
                    private_json(directory / (name + '.private.json'), result)
                    summary[name] = {'http': 'success', 'businessResult': result.get('Result')}
                except Exception as error:
                    summary[name] = {'http': 'failed', 'errorType': type(error).__name__}
            observed = call('/api/v1/device/info', {'serial': serial})
            private_json(directory / 'observation.private.json', observed)
            summary['platformOnline'] = observed.get('Online') is True

        summary.update(session(input_data, directory, on_ready))
        summary['wire'] = inspect_evidence(directory, serial, channel)
        if mode == 'standard' and summary['wire']['registerRequestUserinfo']:
            summary['wire']['passed'] = False
    except Exception as error:
        # Messages may contain URLs, tokens, replies or supplied configuration.
        summary['errorType'] = type(error).__name__
    finally:
        if owned:
            try:
                summary['cleanup'] = cleanup_owned(call, serial)
            except Exception as error:
                summary['cleanup'] = 'unconfirmed: inspection failed'
                summary['cleanupErrorType'] = type(error).__name__
    return summary


def passed(summary):
    return (not summary.get('errorType') and summary.get('exitCode') == 0
            and summary.get('platformOnline') is True and summary.get('wire', {}).get('passed') is True
            and summary.get('deviceinfo', {}).get('businessResult') == 'OK'
            and summary.get('catalog', {}).get('http') == 'success'
            and summary.get('catalog', {}).get('businessResult') in (None, 'OK')
            and summary.get('cleanup') in ('removed', 'absent'))


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--run', action='store_true', help='explicitly enable external test')
    parser.add_argument('--transport', choices=('UDP', 'TCP'), required=True)
    parser.add_argument('--mode', choices=('standard', 'compatibility'), default='standard',
                        help='explicit protocol mode; standard never falls back automatically')
    args = parser.parse_args(argv)
    if not args.run:
        parser.error('--run is required after independent validation')
    try:
        values = load_env()
        validate(values)
    except ConfigError as error:
        print('failed: ' + str(error), file=sys.stderr)
        return 2
    directory = ROOT / 'artifacts/interoperability' / (
        time.strftime('%Y%m%d-%H%M%S', time.gmtime()) + '-' + args.transport.lower() + '-' + args.mode + '-' + secrets.token_hex(3))
    protect_directory(directory)
    client = PlatformClient(values)
    summary = {'errorType': 'LoginUnavailable', 'cleanup': 'not-run'}
    try:
        login = client('/api/v1/login', {
            'username': values['TEST_LIVEGBS_USERNAME'],
            'password': hashlib.md5(values['TEST_LIVEGBS_PASSWORD'].encode()).hexdigest(),
            'token_timeout': '600'}, True)
        client.token = login['URLToken']
        if not isinstance(client.token, str) or not client.token:
            raise ConfigError('test API login did not return a session')
        summary = execute_run(values, args.transport, directory, client, mode=args.mode)
    except Exception as error:
        summary['errorType'] = type(error).__name__
    finally:
        if client.token:
            try:
                client('/api/v1/logout')
            except Exception:
                summary['logout'] = 'unconfirmed; session lifetime limited to 600 seconds'
        private_json(directory / 'summary.json', summary)
        print(json.dumps(summary, ensure_ascii=False))
    if summary.get('errorType'):
        return 2
    return 0 if passed(summary) else 1


if __name__ == '__main__':
    sys.exit(main())
