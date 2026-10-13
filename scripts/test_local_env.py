"""Security and precedence regression tests use synthetic temporary inputs only."""
import contextlib
import io
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import local_env


class LocalEnvTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.example = local_env.ROOT / '.env.example'
        (self.root / '.env.example').write_text(self.example.read_text(), encoding='utf-8')

    def test_precedence_and_literal_values(self):
        (self.root / '.env').write_text('TEST_LIVEGBS_PASSWORD=base\nLAB_PROFILE=local-test\n')
        (self.root / '.env.local').write_text('TEST_LIVEGBS_PASSWORD="$(ignored) $HOME # literal"\n')
        self.assertEqual(local_env.load_env(self.root, {})['TEST_LIVEGBS_PASSWORD'],
                         '$(ignored) $HOME # literal')
        self.assertEqual(local_env.load_env(self.root, {'TEST_LIVEGBS_PASSWORD': 'process'})
                         ['TEST_LIVEGBS_PASSWORD'], 'process')
        self.assertEqual(local_env.load_env(self.root, {'TEST_LIVEGBS_PASSWORD': ''})
                         ['TEST_LIVEGBS_PASSWORD'], '')

    def test_invalid_syntax_never_echoes_values(self):
        for data in ('secret input', 'X=secret\nX=other', 'X="secret', 'X=secret\x00'):
            path = self.root / '.env.local'
            path.write_text(data)
            with self.assertRaises(local_env.ConfigError) as caught:
                local_env.parse_env(path)
            self.assertNotIn('secret', str(caught.exception))

    def test_unknown_key_and_missing_profile(self):
        (self.root / '.env.local').write_text('VITE_PASSWORD=secret')
        with self.assertRaises(local_env.ConfigError):
            local_env.load_env(self.root, {})
        with self.assertRaises(local_env.ConfigError):
            local_env.validate({})

    def test_example_validates_without_auth_inference(self):
        values = local_env.parse_env(self.example)
        local_env.validate(values)
        self.assertEqual(values['TEST_LIVEGBS_SIP_AUTH_MODE'], 'unknown')
        values['TEST_LIVEGBS_PASSWORD'] = 'web-only'
        values['TEST_LIVEGBS_SIP_AUTH_MODE'] = 'digest'
        with self.assertRaises(local_env.ConfigError):
            local_env.validate(values)

    def test_reject_remote_targets_and_url_tricks(self):
        for value in ('http://example.com', 'http://127.0.0.1@192.0.2.1',
                      'http://user:secret@127.0.0.1', 'http://127.0.0.1/path',
                      'http://127.0.0.1?x=1', 'file:///tmp/x', 'http://127.0.0.1:99999',
                      'http://localhost:10000', 'http://[::1]:10000', 'http://127.0.0.1:0'):
            values = local_env.parse_env(self.example)
            values['TEST_LIVEGBS_HTTP_URL'] = value
            with self.assertRaises(local_env.ConfigError) as caught:
                local_env.validate(values)
            self.assertNotIn('secret', str(caught.exception))

    def test_ports_hosts_ids_and_auth(self):
        cases = {'LAB_HTTP_BIND': ['0.0.0.0:18081', '127.0.0.1:0', '127.0.0.1:15060'],
                 'TEST_LIVEGBS_SIP_HOST': ['192.0.2.1'],
                 'TEST_LIVEGBS_SIP_ID': ['123', '１' * 20],
                 'TEST_LIVEGBS_SIP_AUTH_MODE': ['auto'],
                 'LAB_SIP_TRANSPORT': ['TLS']}
        for key, inputs in cases.items():
            for value in inputs:
                values = local_env.parse_env(self.example)
                values[key] = value
                with self.assertRaises(local_env.ConfigError):
                    local_env.validate(values)

    def test_default_command_never_opens_network_or_prints_secret(self):
        values = local_env.parse_env(self.example)
        values['TEST_LIVEGBS_PASSWORD'] = 'synthetic-secret'
        output = io.StringIO()
        with patch.object(local_env, 'load_env', return_value=values), \
                patch.object(local_env.socket, 'create_connection') as connect, \
                patch.object(local_env.socket, 'socket') as socket, \
                contextlib.redirect_stdout(output):
            self.assertEqual(local_env.main([]), 0)
        connect.assert_not_called()
        socket.assert_not_called()
        self.assertNotIn('synthetic-secret', output.getvalue())

    def test_probe_failure_does_not_echo_exception_data(self):
        values = local_env.parse_env(self.example)
        bindings, target = local_env.validate(values)
        output = io.StringIO()
        with patch.object(local_env.socket, 'create_connection', side_effect=OSError('secret')), \
                patch.object(local_env.socket, 'socket', side_effect=OSError('secret')), \
                contextlib.redirect_stdout(output):
            self.assertFalse(local_env.probe(bindings, target))
        self.assertNotIn('secret', output.getvalue())
        self.assertIn('unreachable', output.getvalue())

    def test_probe_closes_resources_and_sends_no_payload(self):
        values = local_env.parse_env(self.example)
        bindings, target = local_env.validate(values)
        with patch.object(local_env.socket, 'create_connection') as connect, \
                patch.object(local_env.socket, 'socket') as socket, \
                contextlib.redirect_stdout(io.StringIO()):
            self.assertTrue(local_env.probe(bindings, target))
        connect.assert_called_once_with(target, timeout=2)
        connect.return_value.__exit__.assert_called_once()
        connect.return_value.__enter__.return_value.sendall.assert_not_called()
        self.assertEqual(socket.return_value.__exit__.call_count, 6)
        socket.return_value.__enter__.return_value.sendto.assert_not_called()


if __name__ == '__main__':
    unittest.main()
