"""Synthetic regressions for the opt-in runner; no personal platform is contacted."""
import contextlib
from datetime import datetime, timedelta, timezone
import hashlib
import io
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import Mock, patch

import interop_evidence
import local_env
import test_livegbs as runner

SERIAL = '11010100001180000001'
CHANNEL = '11010100001310000001'
PLATFORM = '11010100002000000001'


class RunnerTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.directory = Path(temporary.name)
        self.values = local_env.parse_env(local_env.ROOT / '.env.example')
        self.values['TEST_LIVEGBS_SIP_PASSWORD'] = 'env-device-secret'
        self.base = {'Serial': PLATFORM, 'Realm': '1101010000', 'Port': 15060,
                     'DevicePassword': 'api-device-secret'}

    def test_supplied_password_wins_even_if_discovery_masks_it(self):
        for discovered in ('api-device-secret', '******', None):
            base = dict(self.base, DevicePassword=discovered)
            result = runner.resolve_binding(self.values, base, SERIAL)
            self.assertEqual(result['password'], 'env-device-secret')
            self.assertEqual(result['username'], SERIAL)
        self.values['TEST_LIVEGBS_SIP_USERNAME'] = 'explicit-account'
        self.assertEqual(runner.resolve_binding(self.values, self.base, SERIAL)['username'],
                         'explicit-account')

    def test_explicit_empty_or_masked_password_cannot_fall_back(self):
        for value in ('', '******', '••••'):
            self.values['TEST_LIVEGBS_SIP_PASSWORD'] = value
            with self.assertRaises(local_env.ConfigError):
                runner.resolve_binding(self.values, self.base, SERIAL)
        del self.values['TEST_LIVEGBS_SIP_PASSWORD']
        self.assertEqual(runner.resolve_binding(self.values, self.base, SERIAL)['password'],
                         'api-device-secret')
        with self.assertRaises(local_env.ConfigError):
            runner.resolve_binding(self.values, dict(self.base, DevicePassword='***'), SERIAL)

    def test_discovery_cannot_change_authorized_target_or_disable_auth(self):
        for change in ({'Port': 16060}, {'Serial': SERIAL}, {'Realm': ''}):
            with self.assertRaises(local_env.ConfigError) as error:
                runner.resolve_binding(self.values, dict(self.base, **change), SERIAL)
            self.assertNotIn('secret', str(error.exception))
        self.values['TEST_LIVEGBS_SIP_REALM'] = 'different'
        with self.assertRaises(local_env.ConfigError):
            runner.resolve_binding(self.values, self.base, SERIAL)
        self.values['TEST_LIVEGBS_SIP_REALM'] = ''
        self.values['TEST_LIVEGBS_SIP_AUTH_MODE'] = 'none'
        with self.assertRaises(local_env.ConfigError):
            runner.resolve_binding(self.values, self.base, SERIAL)

    def test_cleanup_after_session_exception_only_removes_owned_offline_device(self):
        devices = [{'ID': 'unrelated', 'Online': False}]
        removed = []
        def call(path, params=None, post=False):
            if path == '/api/v1/getbaseconfig':
                return self.base
            if path == '/api/v1/device/list':
                return {'DeviceList': list(devices)}
            if path == '/api/v1/device/remove':
                self.assertTrue(post)
                removed.append(params['serial'])
                devices[:] = [d for d in devices if d['ID'] != params['serial']]
                return {}
            self.fail('unexpected API')
        owned_id = []
        def session(input_data, directory, on_ready):
            owned_id.append(input_data['DeviceID'])
            devices.append({'ID': input_data['DeviceID'], 'Online': False})
            raise RuntimeError('private token and device secret must not be printed')
        summary = runner.execute_run(self.values, 'UDP', self.directory, call, session)
        self.assertEqual(summary['cleanup'], 'removed')
        self.assertEqual(removed, owned_id)
        self.assertEqual(devices, [{'ID': 'unrelated', 'Online': False}])
        self.assertEqual(summary['errorType'], 'RuntimeError')
        self.assertNotIn('private token', json.dumps(summary))

    def test_cleanup_refuses_online_unknown_and_ambiguous_devices(self):
        for devices in ([{'ID': SERIAL, 'Online': True}], [{'ID': SERIAL}],
                        [{'ID': SERIAL, 'Online': False}] * 2):
            call = Mock(return_value={'DeviceList': devices})
            self.assertTrue(runner.cleanup_owned(call, SERIAL).startswith('unconfirmed'))
            call.assert_called_once()
        with self.assertRaises(local_env.ConfigError):
            runner.cleanup_owned(Mock(return_value={}), SERIAL)

    def test_existing_identity_does_not_become_owned(self):
        serial = '3402000000118' + '1000000'
        def call(path, params=None, post=False):
            return self.base if path.endswith('getbaseconfig') else {
                'DeviceList': [{'ID': serial, 'Online': False}]}
        session = Mock()
        with patch.object(runner.secrets, 'randbelow', return_value=0):
            summary = runner.execute_run(self.values, 'UDP', self.directory, call, session)
        session.assert_not_called()
        self.assertEqual(summary['cleanup'], 'not-run')

    def test_http_success_alone_and_unknown_observations_do_not_pass(self):
        summary = {'exitCode': 0, 'platformOnline': True, 'wire': {'passed': True},
                   'catalog': {'http': 'success'},
                   'deviceinfo': {'businessResult': 'OK'}, 'cleanup': 'absent'}
        self.assertTrue(runner.passed(summary))
        for change in ({'platformOnline': None}, {'wire': {'passed': False}},
                       {'catalog': {'http': 'success', 'businessResult': 'ERROR'}},
                       {'cleanup': 'unconfirmed'}, {'exitCode': 1}):
            self.assertFalse(runner.passed(dict(summary, **change)))

    def test_redirects_response_budgets_and_json_shape(self):
        with self.assertRaises(local_env.ConfigError):
            runner.NoRedirect().redirect_request(None)
        client = runner.PlatformClient(self.values)
        for raw in (b'[]', b'x' * (4 * 1024 * 1024 + 1)):
            response = Mock()
            response.__enter__ = Mock(return_value=response)
            response.__exit__ = Mock()
            response.read.return_value = raw
            client.opener = Mock()
            client.opener.open.return_value = response
            with self.assertRaises(local_env.ConfigError):
                client('/api/v1/device/list')
            response.read.assert_called_once_with(4 * 1024 * 1024 + 1)
        response.read.return_value = b'"OK"'
        self.assertEqual(client('/api/v1/logout'), {})
        with self.assertRaises(local_env.ConfigError):
            client('/api/v1/device/list')

    def test_failed_observation_cooperatively_stops_the_direct_test_binary(self):
        process = Mock()
        process.poll.return_value = None
        def launch(*args, **kwargs):
            (self.directory / 'ready').write_text('registered')
            self.assertNotIn('TEST_PRIVATE_SECRET', kwargs['env'])
            self.assertIn('interop.test', args[0][0])
            return process
        with patch.object(runner.subprocess, 'run'), \
                patch.object(runner.subprocess, 'Popen', side_effect=launch), \
                patch.dict(runner.os.environ, {'TEST_PRIVATE_SECRET': 'synthetic'}), \
                contextlib.redirect_stdout(io.StringIO()):
            with self.assertRaises(RuntimeError):
                runner.run_session({}, self.directory, Mock(side_effect=RuntimeError('private')))
        self.assertTrue((self.directory / 'stop').exists())
        process.wait.assert_called_once_with(timeout=20)
        process.terminate.assert_not_called()


class WireEvidenceTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.directory = Path(temporary.name)
        self.records = []
        self.raw = []
        self.origin = datetime(2026, 10, 11, tzinfo=timezone.utc)
        self.fixture()

    def append(self, data, direction, seconds):
        self.raw.append(data)
        self.records.append({'direction': direction, 'at': (self.origin + timedelta(seconds=seconds)).isoformat(),
                             'length': len(data), 'sha256': hashlib.sha256(data).hexdigest()})

    def exchange(self, label, seconds, direction='out', xml='', expires=None, auth=False, status=200):
        method = 'REGISTER' if expires is not None else 'MESSAGE'
        body = xml.encode('gb18030')
        headers = (f'Via: SIP/2.0/UDP 127.0.0.1:15061;branch=z9hG4bK-{label}\r\n'
                   f'From: <sip:{SERIAL}@1101010000>;tag=fixture\r\n'
                   f'To: <sip:{SERIAL}@1101010000>\r\n'
                   f'Call-ID: {label}\r\nCSeq: 1 {method}\r\n')
        additions = (f'Expires: {expires}\r\n' if expires is not None else '')
        if auth:
            additions += 'Authorization: Digest synthetic\r\n'
        request = (f'{method} sip:127.0.0.1:15060 SIP/2.0\r\n' + headers + additions
                   + f'Content-Length: {len(body)}\r\n\r\n').encode() + body
        self.append(request, direction, seconds)
        challenge = 'WWW-Authenticate: Digest realm="fixture",nonce="fixture"\r\n' if status == 401 else ''
        response = (f'SIP/2.0 {status} Test\r\n' + headers + challenge + 'Content-Length: 0\r\n\r\n').encode()
        self.append(response, 'in' if direction == 'out' else 'out', seconds + 0.01)

    def xml(self, root, command, sn, tail=''):
        return (f'<?xml version="1.0" encoding="GB18030"?><{root}><CmdType>{command}</CmdType>'
                f'<SN>{sn}</SN><DeviceID>{SERIAL}</DeviceID>{tail}</{root}>')

    def fixture(self):
        self.exchange('challenge', 0, expires=86400, status=401)
        self.exchange('register', 0.1, expires=86400, auth=True)
        self.exchange('refresh', 0.15, expires=86400, auth=True)
        for i in range(1, 4):
            self.exchange('heartbeat' + str(i), 0.2 + i * 60,
                          xml=self.xml('Notify', 'Keepalive', str(i), '<Status>OK</Status>'))
        for index, cmd in enumerate(('DeviceInfo', 'Catalog')):
            self.exchange('query' + cmd, 1 + index, 'in', self.xml('Query', cmd, '10'))
            tail = ('<Result>OK</Result><DeviceName>中文𠀀</DeviceName><Channel>1</Channel>'
                    if cmd == 'DeviceInfo' else '<SumNum>2</SumNum><DeviceList Num="2">'
                    f'<Item><DeviceID>{SERIAL}</DeviceID></Item>'
                    f'<Item><DeviceID>{CHANNEL}</DeviceID></Item></DeviceList>')
            self.exchange('response' + cmd, 1.2 + index, xml=self.xml('Response', cmd, '10', tail))
        self.exchange('unregister', 181, expires=0, auth=True)

    def write(self):
        for i, raw in enumerate(self.raw):
            (self.directory / f'{i:06d}.sip').write_bytes(raw)
        (self.directory / 'evidence.json').write_text(
            json.dumps({'traces': {'records': self.records, 'dropped': 0}}), encoding='utf-8')

    def result(self):
        self.write()
        return interop_evidence.inspect_evidence(self.directory, SERIAL, CHANNEL)

    def test_complete_independent_exchanges_and_normal_cycles(self):
        result = self.result()
        self.assertTrue(result['passed'], result)
        self.assertEqual(result['heartbeatAcknowledged'], 3)
        self.assertNotIn(SERIAL, json.dumps(result))

    def test_fingerprint_and_length_tampering_are_rejected(self):
        self.write()
        path = self.directory / '000000.sip'
        path.write_bytes(path.read_bytes() + b'X')
        with self.assertRaises(interop_evidence.EvidenceError):
            interop_evidence.inspect_evidence(self.directory, SERIAL, CHANNEL)

    def test_sip_200_without_correlated_business_response_is_not_success(self):
        messages = None
        self.write()
        messages = interop_evidence.read_messages(self.directory)
        for message in messages:
            xml = message.get('xml')
            if xml is not None and xml.tag == 'Response' and xml.findtext('CmdType') == 'Catalog':
                xml.find('SN').text = '999'
        result = interop_evidence.assess_messages(messages, SERIAL, CHANNEL)
        self.assertFalse(result['passed'])
        self.assertFalse(result['catalog'])

    def test_duplicate_retransmissions_and_accelerated_clocks_do_not_fake_three_cycles(self):
        self.write()
        messages = interop_evidence.read_messages(self.directory)
        heartbeats = [m for m in messages if m.get('xml') is not None
                      and m['xml'].findtext('CmdType') == 'Keepalive']
        one = heartbeats[0]
        remove = {m['key'] for m in heartbeats[1:]}
        duplicate = [m for m in messages if m['key'] not in remove] + [one, one]
        self.assertFalse(interop_evidence.assess_messages(duplicate, SERIAL, CHANNEL)['normalHeartbeatCycles'])
        for i, message in enumerate(heartbeats):
            message['at'] = self.origin + timedelta(seconds=i + 1)
        self.assertFalse(interop_evidence.assess_messages(messages, SERIAL, CHANNEL)['normalHeartbeatCycles'])

    def test_catalog_duplicate_identity_and_unacknowledged_response_fail(self):
        self.write()
        messages = interop_evidence.read_messages(self.directory)
        response = next(m for m in messages if m.get('xml') is not None
                        and m['xml'].tag == 'Response' and m['xml'].findtext('CmdType') == 'Catalog')
        response['xml'].findall('./DeviceList/Item/DeviceID')[1].text = SERIAL
        self.assertFalse(interop_evidence.assess_messages(messages, SERIAL, CHANNEL)['catalog'])
        self.write()
        messages = interop_evidence.read_messages(self.directory)
        messages = [m for m in messages if not ('status' in m and m['key'] == response['key'])]
        self.assertFalse(interop_evidence.assess_messages(messages, SERIAL, CHANNEL)['catalog'])


if __name__ == '__main__':
    unittest.main()
