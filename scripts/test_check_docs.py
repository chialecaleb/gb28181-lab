"""Exercise rejection behavior against disposable Git repositories."""
import contextlib
import io
import json
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import check_docs


class DocumentationGateTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name).resolve()
        subprocess.run(['git', 'init', '-q', str(self.root)], check=True)
        (self.root / 'docs/governance').mkdir(parents=True)
        self.write('README.md', '# Entry\n\n[Section](#section)\n\n## Section\n')
        entry = dict(path='README.md', kind='entry', status='active', owner='maintainer',
                     scope='entry', reviewed='2026-10-07', basis='test')
        self.write('docs/governance/document-registry.json', json.dumps({'documents': [entry]}))
        subprocess.run(['git', '-C', str(self.root), 'add', '.'], check=True)

    def write(self, path, text):
        (self.root / path).write_text(text, encoding='utf-8')

    def run_gate(self):
        with patch.object(check_docs, 'ROOT', self.root), contextlib.redirect_stdout(io.StringIO()) as output:
            code = check_docs.main()
        return code, output.getvalue()

    def test_valid(self):
        self.assertEqual(self.run_gate()[0], 0)

    def test_missing_registration(self):
        self.write('extra.md', '# Extra\n')
        subprocess.run(['git', '-C', str(self.root), 'add', 'extra.md'], check=True)
        code, output = self.run_gate()
        self.assertEqual(code, 1)
        self.assertIn('Unregistered', output)

    def test_bad_links_and_paths(self):
        for text, expected in [('[missing](gone.md)', 'Broken link'),
                               ('[missing](#gone)', 'Broken anchor'),
                               ('[outside](../outside.md)', 'escapes repository'),
                               ('Local C:/private/sample', 'absolute path')]:
            with self.subTest(expected=expected):
                self.write('README.md', '# Entry\n' + text)
                code, output = self.run_gate()
                self.assertEqual(code, 1)
                self.assertIn(expected, output)


if __name__ == '__main__':
    unittest.main()
